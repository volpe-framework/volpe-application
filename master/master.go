package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
	ccomms "volpe-framework/comms/common"
	vcomms "volpe-framework/comms/volpe"
	cm "volpe-framework/container_mgr"

	// "volpe-framework/metrics"
	"volpe-framework/scheduler"

	apilib "volpe-framework/master/api"

	model "volpe-framework/master/model"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// TODO: reenable when required
	// metrics.InitOTelSDK()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	masterContext, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	sched, err := scheduler.NewPrelimScheduler()
	if err != nil {
		log.Error().Caller().Msgf("err with sched: %s", err.Error())
		panic(err)
	}

	eventChannel := make(chan string, 5)

	sched.Init()

	port, ok := os.LookupEnv("VOLPE_PORT")
	if !ok {
		log.Warn().Caller().Msgf("using default VOLPE_PORT of 8080")
		port = "8080"
	}
	portD := uint16(0)
	fmt.Sscan(port, &portD)

	cman := cm.NewContainerManager(false)

	metricChan := make(chan *vcomms.DeviceMetricsMessage, 10)
	popChan := make(chan *ccomms.Population, 10)

	problemStore, _ := model.NewProblemStore()

	mc, err := vcomms.NewMasterComms(portD, metricChan, popChan, sched, problemStore, eventChannel)
	if err != nil {
		log.Fatal().Caller().Msgf("error initializing master comms: %s", err.Error())
		panic(err)
	}


	api, err := apilib.NewVolpeAPI(problemStore, sched, cman, eventChannel)
	if err != nil {
		panic(err)
	}

	apilib.RunAPI(8000, api)
	log.Info().Caller().Msgf("master API listening on port %d", 8000)

	schedule := make(scheduler.Schedule)
	var schedMutex sync.Mutex

	go sendMetric(metricChan, eventChannel, sched)

	go processContainerMetrics(cman, problemStore, masterContext)

	go recvPopulation(cman, popChan)

	go applySchedule(sched, schedule, &schedMutex)

	go sendPopulation(mc, cman, schedule, &schedMutex)

	mc.Serve()
}

func processContainerMetrics(contman *cm.ContainerManager, problemStore *model.ProblemStore, masterContext context.Context) {
	 metricChan := make(chan *cm.ContainerMetrics, 5)
	contman.StreamContainerMetrics(metricChan, masterContext)
	for {
		metric := <- metricChan
		log.Info().Msgf("Container for %s using %f GB memory", metric.ProblemID, metric.MemUsageGB)
		// problemStore.UpdateMemory(problemID, metric.MemUsageGB)
	}
}

func recvPopulation(cman *cm.ContainerManager, popChan chan *ccomms.Population) {
	for {
		m, ok := <-popChan
		if !ok {
			log.Error().Caller().Msg("popChan closed")
			break
		}
		log.Info().Msgf("received population for problem %s", m.GetProblemID())
		err := cman.IncorporatePopulation(m)
		if err != nil {
			log.Err(err).Msgf("could not incorporate population %s", m.GetProblemID())
		}
	}
}

func sendMetric(metricChan chan *vcomms.DeviceMetricsMessage, eventChannel chan string, sched scheduler.Scheduler) {
	for {
		m, ok := <-metricChan
		if !ok {
			log.Error().Caller().Msg("metricChan closed")
			return
		}
		sched.UpdateMetrics(m)

		jsonMsg, _ := json.Marshal(map[string]any{
			"type": "WorkerMetrics",
			"workerID": m.GetWorkerID(),
			"cpuUtilPerc": m.GetCpuUtilPerc(),
			"memUsageGB": m.GetMemUsageGB(),
		})
		eventChannel <- string(jsonMsg)
	}
}

func sendPopulation(master *vcomms.MasterComms, cman *cm.ContainerManager, schedule scheduler.Schedule, schedMutex *sync.Mutex) {
	for {
		schedMutex.Lock()

		schedule.Apply(func (workerID string, problemID string, val int32) {
			adjpop := &vcomms.AdjustInstancesMessage{
				ProblemID: problemID,
				Seed: nil, 
				Instances: val,
			}
			if val != 0 {
				// TODO: pop sizes parameter
				subpop, err := cman.GetRandomSubpopulation(problemID, 10*int(val))
				if err != nil {
					log.Error().Caller().Msgf("error getting subpop wID %s pID %s to update schedule: %s", workerID, problemID, err.Error())
					return
				}
				adjpop.Seed = subpop
			}
			msg := vcomms.MasterMessage{
				Message: &vcomms.MasterMessage_AdjInst{
					AdjInst: adjpop,
				},
			}
			log.Info().Caller().Msgf("worker %s problem %s pop %d", workerID, problemID, val)
			err := master.SendPopulationSize(workerID, &msg)
			if err != nil {
				log.Error().Caller().Msgf("error pushing subpop wID %s pID %s: %s", workerID, problemID, err.Error())
				return
			}
		})

		schedMutex.Unlock()
		log.Debug().Msg("Sent schedule")
		time.Sleep(5*time.Second)
	}
}

func applySchedule(sched scheduler.Scheduler, schedule scheduler.Schedule, schedMutex *sync.Mutex) {
	for {
		schedMutex.Lock()
		err := sched.FillSchedule(schedule)
		if err != nil {
			log.Error().Caller().Msgf("error filling sched: %s", err.Error())
			return
		}
		/* Moved applying changes to diff call
		schedule.Apply(func (workerID string, problemID string, val int32) {
			adjpop := &vcomms.AdjustInstancesMessage{
				ProblemID: problemID,
				Seed: nil, 
				Instances: val,
			}
			if val != 0 {
				subpop, err := cman.GetRandomSubpopulation(problemID, val)
				if err != nil {
					log.Error().Caller().Msgf("error getting subpop wID %s pID %s to update schedule: %s", workerID, problemID, err.Error())
					return
				}
				adjpop.Seed = subpop
			}
			msg := vcomms.MasterMessage{
				Message: &vcomms.MasterMessage_AdjInst{
					AdjInst: adjpop,
				},
			}
			log.Info().Caller().Msgf("worker %s problem %s pop %d", workerID, problemID, val)
			err = master.SendPopulationSize(workerID, &msg)
			if err != nil {
				log.Error().Caller().Msgf("error pushing subpop wID %s pID %s: %s", workerID, problemID, err.Error())
				return
			}
		})
		*/
		log.Info().Caller().Msg("Modified schedule")
		schedMutex.Unlock()
		time.Sleep(5*time.Second)
	}
}
