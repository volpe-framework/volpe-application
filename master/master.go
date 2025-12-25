package main

import (
	"fmt"
	"os"
	"time"
	ccomms "volpe-framework/comms/common"
	vcomms "volpe-framework/comms/volpe"
	cm "volpe-framework/container_mgr"

	// "volpe-framework/metrics"
	"volpe-framework/scheduler"

	apilib "volpe-framework/master/api"

	model "volpe-framework/master/model"

	"github.com/rs/zerolog/log"
)

func main() {
	// TODO: reenable when required
	// metrics.InitOTelSDK()

	sched, err := scheduler.NewPrelimScheduler()
	if err != nil {
		log.Error().Caller().Msgf("err with sched: %s", err.Error())
		panic(err)
	}

	sched.Init()

	port, ok := os.LookupEnv("VOLPE_PORT")
	if !ok {
		log.Warn().Caller().Msgf("using default VOLPE_PORT of 8080")
		port = "8080"
	}
	portD := uint16(0)
	fmt.Sscan(port, &portD)

	cman := cm.NewContainerManager(false)

	metricChan := make(chan *vcomms.MetricsMessage, 10)
	popChan := make(chan *ccomms.Population, 10)

	problemStore, _ := model.NewProblemStore()

	mc, err := vcomms.NewMasterComms(portD, metricChan, popChan, sched, problemStore)
	if err != nil {
		log.Fatal().Caller().Msgf("error initializing master comms: %s", err.Error())
		panic(err)
	}


	api, err := apilib.NewVolpeAPI(problemStore, sched, cman)
	if err != nil {
		panic(err)
	}

	apilib.RunAPI(8000, api)
	log.Info().Caller().Msgf("master API listening on port %d", 8000)

	go sendMetric(metricChan, sched)

	go recvPopulation(cman, popChan)

	go applySchedule(mc, cman, sched)

	mc.Serve()
}

func recvPopulation(cman *cm.ContainerManager, popChan chan *ccomms.Population) {
	for {
		m, ok := <-popChan
		if !ok {
			log.Error().Caller().Msg("popChan closed")
			break
		}
		cman.IncorporatePopulation(m)
		log.Info().Caller().Msgf("received population for problem %s", m.GetProblemID())
		fmt.Println(m.GetProblemID() + " ", m.Members[0].Fitness)
	}
}

func sendMetric(metricChan chan *vcomms.MetricsMessage, sched scheduler.Scheduler) {
	for {
		m, ok := <-metricChan
		if !ok {
			log.Error().Caller().Msg("metricChan closed")
			return
		}
		sched.UpdateMetrics(m)
	}
}
 
func applySchedule(master *vcomms.MasterComms, cman *cm.ContainerManager, sched scheduler.Scheduler) {
	// TODO: optimise by calculating deltas
	schedule := make(scheduler.Schedule)
	for {
		err := sched.FillSchedule(schedule)
		if err != nil {
			log.Error().Caller().Msgf("error filling sched: %s", err.Error())
			return
		}
		schedule.Apply(func (workerID string, problemID string, val int32) {
			adjpop := &vcomms.AdjustInstancesMessage{
				ProblemID: problemID,
				Seed: nil, 
				Instances: val,
			}
			if val != 0 {
				subpop, err := cman.GetRandomSubpopulation(problemID)
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
			log.Info().Caller().Msgf("worker %s problem %s pop %s", workerID, problemID, val)
			err = master.SendPopulationSize(workerID, &msg)
			if err != nil {
				log.Error().Caller().Msgf("error pushing subpop wID %s pID %s: %s", workerID, problemID, err.Error())
				return
			}
		})
		log.Info().Caller().Msg("Applied schedule")
		time.Sleep(5*time.Second)
	}
}
