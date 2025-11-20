package main

import (
	"fmt"
	"net"
	"os"
	"time"
	ccomms "volpe-framework/comms/common"
	vcomms "volpe-framework/comms/volpe"
	cm "volpe-framework/container_mgr"
	"volpe-framework/metrics"
	"volpe-framework/scheduler"
	vapi "volpe-framework/comms/api"

	model "volpe-framework/master/model"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

func main() {
	metrics.InitOTelSDK()

	sched, err := scheduler.NewStaticScheduler([]string{"abcd123", "xyzabc"})
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

	cman := cm.NewContainerManager()

	metricChan := make(chan *vcomms.MetricsMessage, 10)
	popChan := make(chan *ccomms.Population, 10)

	mc, err := vcomms.NewMasterComms(portD, metricChan, popChan, sched)
	if err != nil {
		log.Fatal().Caller().Msgf("error initializing master comms: %s", err.Error())
		panic(err)
	}

	problemStore, _ := model.NewProblemStore()

	api, err := NewVolpeAPI(problemStore, sched, cman)
	if err != nil {
		panic(err)
	}

	serv := grpc.NewServer()
	lis, err := net.Listen("tcp", "0.0.0.0:8000")
	if err != nil {
		panic(err)
	}
	log.Info().Caller().Msgf("master API listening on port %d", 8000)
	vapi.RegisterVolpeAPIServer(serv, api)

	go serv.Serve(lis)

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
			adjpop := &vcomms.AdjustPopulationMessage{
				ProblemID: problemID,
				Seed: nil, 
				Size: val,
			}
			if val != 0 {
				subpop, err := cman.GetSubpopulation(problemID)
				if err != nil {
					log.Error().Caller().Msgf("error getting subpop wID %s pID %s to update schedule: %s", workerID, problemID, err.Error())
					return
				}
				adjpop.Seed = subpop
			}
			msg := vcomms.MasterMessage{
				Message: &vcomms.MasterMessage_AdjPop{
					AdjPop: adjpop,
				},
			}
			err = master.SendPopulationSize(workerID, &msg)
			if err != nil {
				log.Error().Caller().Msgf("error pushing subpop wID %s pID %s: %s", workerID, problemID, err.Error())
				return
			}
		})
		time.Sleep(30*time.Second)
	}
}
