package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"runtime"
	"time"

	"volpe-framework/comms/volpe"
	vcomms "volpe-framework/vcomms"
	pcomms "volpe-framework/comms/volpe"
	contman "volpe-framework/container_mgr"
	"volpe-framework/model"
	"volpe-framework/types"

	loadstat "github.com/mackerelio/go-osstat/loadavg"
	memorystat "github.com/mackerelio/go-osstat/memory"
	networkstat "github.com/mackerelio/go-osstat/network"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"flag"
)

func main() {
	// TODO: reenable when required
	// metrics.InitOTelSDK()

	configPath := ""

	flag.StringVar(&configPath, "config-path", "", "Location of the .ini config file for the VolPE worker")

	flag.Parse()

	workerConfig, err := LoadConfig(configPath)
	if err != nil {
		log.Err(err).Msgf("failed to load config from \"%s\"", configPath)
		return
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	workerContext, cancelFunc := context.WithCancel(context.TODO())
	defer cancelFunc()

	problemStore, err := model.NewProblemStore()
	if err != nil {
		log.Err(err).Msgf("failed to initialize problem store")
		return
	}

	volpeMaster := workerConfig.GeneralConfig.VolPEMaster

	immigChan := make(chan *volpe.MigrationMessage, 10)

	if volpeMaster == "" {
		log.Warn().Msgf("VolPE master not found in config, loading from env variable VOLPE_MASTER")

		var ok bool
		volpeMaster, ok = os.LookupEnv("VOLPE_MASTER")
		if !ok {
			log.Warn().Msgf("using default VOLPE_MASTER of localhost:8080")
			volpeMaster = "localhost:8080"
		}
	}

	workerID := workerConfig.GeneralConfig.WorkerID

	if workerID == "" {
		workerID = "worker_" + fmt.Sprintf("%d", rand.Int())
		log.Warn().Caller().Msgf("Worker ID not found, using %s instead", workerID)
	}

	memoryGB := workerConfig.ResourceConfig.MemoryGB
	if memoryGB == 0 {
		stats, _ := memorystat.Get()
		memoryGB = float32(stats.Total)/(1024*1024*1024)
		log.Warn().Caller().Msgf("Memory allocation not found, using %f GB instead", memoryGB)
	}

	cpuCount := workerConfig.ResourceConfig.CpuCount
	if cpuCount == 0 {
		cpuCount = int32(runtime.NumCPU())
		log.Warn().Caller().Msgf("CPU count not found, using %d CPUs instead", cpuCount)
	}

	wEmigChan := make(chan *volpe.MigrationMessage, 10)

	wc, err := vcomms.NewWorkerComms(volpeMaster, workerID, memoryGB, cpuCount)
	if err != nil {
		log.Fatal().Caller().Msgf("could not create workercomms: %s", err.Error())
		panic(err)
	}
	cm := contman.NewWorkerContainerManager(workerContext, problemStore, wEmigChan)

	go deviceMetricsExporter(workerContext, wc)

	go emigrationHandler(wc, wEmigChan)

	// go cm.RunMetricsExport(wc, workerID)

	// TODO: reevaluate if worker container metrics are needed
	// metricsChan := make(chan *contman.ContainerMetrics, 5)
	// go cm.StreamContainerMetrics(metricsChan, workerContext)

	adjInstChan := make(chan *pcomms.AdjustInstancesMessage, 10)

	go adjInstHandler(wc, adjInstChan, cm, problemStore)

	go immigrationHandler(cm, immigChan)
	
	wc.HandleStreams(adjInstChan, immigChan)
}

func emigrationHandler(wc *vcomms.WorkerComms, wEmigChan chan *volpe.MigrationMessage) {
	for {
		pop, ok := <- wEmigChan
		if !ok {
			log.Error().Msgf("Exiting emigration handler")
			return
		}
		log.Info().Msgf("Sending emigration for problemID %s from worker %s, container %d", pop.GetPopulation().GetProblemID(), pop.GetWorkerID(), pop.GetContainerID())
		wc.SendSubPopulation(pop)
	}
}

func immigrationHandler(cm *contman.ContainerManager, immigChan chan *volpe.MigrationMessage) {
	for {
		pop, ok := <- immigChan
		if !ok {
			log.Error().Msgf("Exiting immigration handler")
		}
		err := cm.IncorporatePopulation(pop)
		if err != nil {
			log.Err(err).Msgf("failed to handle immigration")
		}
	}
}

// TODO: rewrite handler based on any aggregation of metrics needed
// func workerMetricsHandler(metricChan chan *contman.ContainerMetrics, wc *vcomms.WorkerComms) {
// 	for {
// 		metrics, ok := <- metricChan 
// 		if !ok {
// 			log.Info().Msgf("Metrics channel closed, exiting metrics handler")
// 			return
// 		}
// 		log.Debug().Msgf("Container %s using %f GB memory", metrics.ContainerName, metrics.MemUsageGB)
// 	}
// }

func deviceMetricsExporter(ctx context.Context, wc *vcomms.WorkerComms) {
	memStats, err := memorystat.Get()
	memGB := float32(0)
	if err != nil {
		log.Err(err).Msgf("Failed fetching memory stats")
	} else {
		memGB = float32(memStats.Used)/(1024*1024*1024)
	}

	loadStats, err := loadstat.Get()
	cpuPerc := float32(0)
	if err != nil {
		log.Err(err).Msgf("Failed to fetch CPU stats")
	} else {
		cpuPerc = float32(loadStats.Loadavg5)
	}

	netTxBytes := uint32(0)
	netRxBytes := uint32(0)
	netStats, err := networkstat.Get()
	if err != nil {
		log.Err(err).Msgf("Failed to fetch network stats")
	} else {
		for _, netStat := range(netStats) {
			netRxBytes += uint32(netStat.RxBytes)
			netTxBytes += uint32(netStat.TxBytes)
		}
	}

	for ctx.Err() == nil {
		wc.SendDeviceMetrics(&pcomms.DeviceMetricsMessage{
			CpuUtilPerc: cpuPerc,
			MemUsageGB: memGB,
			NetTxBytes: netTxBytes,
			NetRxBytes: netRxBytes,
		})
		time.Sleep(5*time.Second)
	}
	log.Info().Msgf("Stopping device metrics export")
}

func adjInstHandler(wc *vcomms.WorkerComms, adjInstChan chan *pcomms.AdjustInstancesMessage, cm *contman.ContainerManager, probStore *model.ProblemStore) {
	for {
		adjInst, ok := <- adjInstChan
		if !ok {
			log.Info().Caller().Msg("adjPopChan closed")
			return
		}
		problemID := adjInst.GetProblemID()

		_, ok = probStore.GetFileName(problemID)
		if !ok {
			meta := types.Problem{}
			err := wc.GetProblemData(problemID, &meta)
			if err != nil {
				log.Error().Caller().Msgf("error fetching problemID %s: %s", problemID, err.Error())
				continue
			}
			probStore.NewProblem(meta)
			err = probStore.RegisterImage(problemID, meta.ImagePath)
			if err != nil {
				log.Err(err).Msgf("error registering image for problemID %s", problemID)
				continue
			}
			log.Debug().Msgf("retrieved image for problemiD %s", problemID)
		}

		if !cm.HasProblem(problemID) {
			err := cm.TrackProblem(problemID)
			if err != nil {
				log.Error().Caller().Msgf("error adding problem %s: %s", problemID, err.Error())
				continue
			}
			log.Info().Caller().Msgf("Added problem %s", problemID)
		}
		err := cm.HandleInstancesEvent(adjInst)
		if err != nil {
			log.Err(err).Msgf("failed to handle instance event for %s, to instances %d", problemID, adjInst.GetInstances())
			continue
		}
	}
}
