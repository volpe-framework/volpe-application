package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
	pcomms "volpe-framework/comms/volpe"
	vcomms "volpe-framework/vcomms"
	cm "volpe-framework/container_mgr"

	"volpe-framework/scheduler"

	apilib "volpe-framework/master/api"

	"volpe-framework/model"

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


	problemStore, _ := model.NewProblemStore()


	emigChan := make(chan *pcomms.MigrationMessage, 10)

	cman := cm.NewMasterContainerManager(masterContext, problemStore, emigChan)

	metricChan := make(chan *pcomms.DeviceMetricsMessage, 10)
	immigChan := make(chan *pcomms.MigrationMessage, 10)



	mc, err := vcomms.NewMasterComms(portD, metricChan, immigChan, sched, problemStore, eventChannel)
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

	// go processContainerMetrics(cman, problemStore, masterContext)

	go recvPopulation(cman, immigChan)

	go calcSchedule(sched, schedule, &schedMutex)

	go sendSchedule(mc, schedule, &schedMutex)

	go sendPopulation(mc, emigChan)

	mc.Serve()
}

// func processContainerMetrics(contman *cm.ContainerManager, problemStore *model.ProblemStore, masterContext context.Context) {
// 	metricChan := make(chan *cm.ContainerMetrics, 5)
// 	contman.StreamContainerMetrics(metricChan, masterContext)
// 	for {
// 		metric := <- metricChan
// 		log.Info().Msgf("Container for %s using %f GB memory", metric.ProblemID, metric.MemUsageGB)
// 		// problemStore.UpdateMemory(problemID, metric.MemUsageGB)
// 	}
// }

func recvPopulation(cman *cm.ContainerManager, popChan chan *pcomms.MigrationMessage) {
	for {
		m, ok := <-popChan
		if !ok {
			log.Error().Caller().Msg("popChan closed")
			break
		}
		log.Info().Msgf("received population for problem %s", m.GetPopulation().GetProblemID())
		err := cman.IncorporatePopulation(m)
		if err != nil {
			log.Err(err).Msgf("could not incorporate population %s", m.GetPopulation().GetProblemID())
		}
	}
}

func sendMetric(metricChan chan *pcomms.DeviceMetricsMessage, eventChannel chan string, sched scheduler.Scheduler) {
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

func sendSchedule(master *vcomms.MasterComms, schedule scheduler.Schedule, schedMutex *sync.Mutex) {
	for {
		schedMutex.Lock()
		schedule.Apply(func (workerID string, problemID string, val int32) {
 			adjinst := &pcomms.AdjustInstancesMessage{
 				ProblemID: problemID,
 				Instances: val,
 			}
 			msg := pcomms.MasterMessage{
 				Message: &pcomms.MasterMessage_AdjInst{
 					AdjInst: adjinst,
 				},
 			}
 			log.Debug().Caller().Msgf("worker %s problem %s instances %d", workerID, problemID, val)
 			err := master.SendMasterMessage(workerID, &msg)
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

func sendPopulation(master *vcomms.MasterComms, emigChan chan *pcomms.MigrationMessage) {
	for {
		mig, ok := <- emigChan
		if !ok {
			log.Error().Msgf("sendPopulation exiting")
			return
		}
		msg := pcomms.MasterMessage{
			Message: &pcomms.MasterMessage_Migration{
				Migration: mig,
			},
		}

		err := master.SendMasterMessage(mig.GetWorkerID(), &msg)
		if err != nil {
			log.Err(err).Msgf("failed to send population to workerID %s", mig.GetWorkerID())
		}
		time.Sleep(5*time.Second)
	}
}

func calcSchedule(sched scheduler.Scheduler, schedule scheduler.Schedule, schedMutex *sync.Mutex) {
	for {
		schedMutex.Lock()
		err := sched.FillSchedule(schedule)
		if err != nil {
			log.Error().Caller().Msgf("error filling sched: %s", err.Error())
			return
		}
		log.Info().Caller().Msg("Modified schedule")
		schedMutex.Unlock()
		time.Sleep(5*time.Second)
	}
}
