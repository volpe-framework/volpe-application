package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	vcomms "volpe-framework/comms/volpe"
	contman "volpe-framework/container_mgr"

	memorystat "github.com/mackerelio/go-osstat/memory"
	loadstat "github.com/mackerelio/go-osstat/loadavg"
	networkstat "github.com/mackerelio/go-osstat/network"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// TODO: reenable when required
	// metrics.InitOTelSDK()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	workerContext, cancelFunc := context.WithCancel(context.TODO())
	defer cancelFunc()

	endpoint, ok := os.LookupEnv("VOLPE_MASTER")
	if !ok {
		log.Warn().Caller().Msgf("using default VOLPE_MASTER of localhost:8080")
		endpoint = "localhost:8080"
	}
	workerID, ok := os.LookupEnv("VOLPE_WORKER_ID")
	if !ok {
		workerID = "worker_" + fmt.Sprintf("%d", rand.Int())
		log.Warn().Caller().Msgf("VOLPE_WORKER_ID not found, using %s instead", workerID)
	}

	wc, err := vcomms.NewWorkerComms(endpoint, workerID)
	if err != nil {
		log.Fatal().Caller().Msgf("could not create workercomms: %s", err.Error())
		panic(err)
	}
	cm := contman.NewContainerManager(true)

	go deviceMetricsExporter(workerContext, wc)

	// go cm.RunMetricsExport(wc, workerID)

	// TODO: reevaluate if worker container metrics are needed
	// metricsChan := make(chan *contman.ContainerMetrics, 5)
	// go cm.StreamContainerMetrics(metricsChan, workerContext)

	go populationExtractor(cm, wc)

	adjInstChan := make(chan *vcomms.AdjustInstancesMessage, 10)

	go adjInstHandler(wc, adjInstChan, cm)
	
	wc.HandleStreams(adjInstChan)
}

// TODO: rewrite handler based on any aggregation of metrics needed
// func workerMetricsHandler(metricChan chan *contman.ContainerMetrics, wc *volpe.WorkerComms) {
// 	for {
// 		metrics, ok := <- metricChan 
// 		if !ok {
// 			log.Info().Msgf("Metrics channel closed, exiting metrics handler")
// 			return
// 		}
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
		wc.SendDeviceMetrics(&vcomms.DeviceMetricsMessage{
			CpuUtilPerc: cpuPerc,
			MemUsageGB: memGB,
			NetTxBytes: netTxBytes,
			NetRxBytes: netRxBytes,
		})
		time.Sleep(5*time.Second)
	}
	log.Info().Msgf("Stopping device metrics export")
}

func populationExtractor(cm *contman.ContainerManager, wc *vcomms.WorkerComms) {
	// extracts popln every X seconds and sends to master
	for {
		// TODO: make this a parameter
		pops, err := cm.GetSubpopulations(10)
		if err == nil {
			for _, pop := range pops {
				err = wc.SendSubPopulation(pop)
				if err != nil {
					log.Error().Caller().Msgf("couldn't send subpop %s: %s",
						pop.GetProblemID(),
						err.Error(),
					)
					continue
				}
				log.Info().Caller().Msgf("sent popln for %s", pop.GetProblemID())
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func adjInstHandler(wc *vcomms.WorkerComms, adjInstChan chan *vcomms.AdjustInstancesMessage, cm *contman.ContainerManager) {
	for {
		adjInst, ok := <- adjInstChan
		if !ok {
			log.Info().Caller().Msg("adjPopChan closed")
			return
		}
		problemID := adjInst.GetProblemID()
		instances := adjInst.GetInstances()
		if instances == 0 {
			if cm.HasProblem(problemID) {
				log.Info().Msgf("stopping problem %s", problemID)
				cm.RemoveProblem(problemID)
			}
			continue
		}
		if !cm.HasProblem(problemID) {
			fname, err := wc.GetImageFile(problemID)
			if err != nil {
				log.Error().Caller().Msgf("error fetching problemID %s: %s", problemID, err.Error())
				continue
			}
			err = cm.AddProblem(problemID, fname, int(adjInst.GetInstances()))
			if err != nil {
				log.Error().Caller().Msgf("error running problem %s: %s", problemID, err.Error())
				continue
			}
			log.Info().Caller().Msgf("Running new problem %s", problemID)
		}
		cm.HandleInstancesEvent(adjInst)
	}
}
