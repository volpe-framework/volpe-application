package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	vcomms "volpe-framework/comms/volpe"
	contman "volpe-framework/container_mgr"
	"volpe-framework/metrics"

	"github.com/rs/zerolog/log"
)

func main() {
	metrics.InitOTelSDK()
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
	cm := contman.NewContainerManager()

	go cm.RunMetricsExport(wc, workerID)

	go populationExtractor(cm, wc)

	// err = cm.AddProblem("problem1", "../comms/pybindings/grpc_test_img.tar")
	// if err != nil {
	// 	log.Fatal().Caller().Msgf("failed to run pod with error: %s", err.Error())
	// 	panic(err)
	// }
	// TODO: stop container
	// defer cm.StopContainer(containerName)

	log.Log().Caller().Msgf("started container at port unknown") // %d", -1)

	adjPopChan := make(chan *vcomms.AdjustPopulationMessage, 10)
	

	go adjPopHandler(wc, adjPopChan, cm)
	
	wc.HandleStreams(adjPopChan)
}

func populationExtractor(cm *contman.ContainerManager, wc *vcomms.WorkerComms) {
	// extracts popln every X seconds and sends to master
	for {
		pops, err := cm.GetSubpopulations()
		if err == nil {
			for _, pop := range pops {
				err = wc.SendSubPopulation(pop)
				if err != nil {
					log.Error().Caller().Msgf("couldn't send subpop %s: %s",
						pop.GetProblemID(),
						err.Error(),
					)
				}
			}
		}
		time.Sleep(30 * time.Second)
	}
}

func adjPopHandler(wc *vcomms.WorkerComms, adjPopChan chan *vcomms.AdjustPopulationMessage, cm *contman.ContainerManager) {
	for {
		adjPop, ok := <- adjPopChan
		if !ok {
			log.Info().Caller().Msg("adjPopChan closed")
			return
		}
		problemID := adjPop.GetProblemID()
		if !cm.HasProblem(problemID) {
			fname, err := wc.GetImageFile(problemID)
			if err != nil {
				log.Error().Caller().Msgf("error fetching problemID %s: %s", problemID, err.Error())
				continue
			}
			err = cm.AddProblem(problemID, fname)
			if err != nil {
				log.Error().Caller().Msgf("error running problem %s: %s", problemID, err.Error())
				continue
			}
		}
		cm.HandlePopulationEvent(adjPop)
	}
}
