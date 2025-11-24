package container_mgr

import (
	"context"
	"errors"
	"sync"
	"volpe-framework/comms/common"
	ccoms "volpe-framework/comms/container"
	"volpe-framework/comms/volpe"

	otelmetric "go.opentelemetry.io/otel/metric"

	"github.com/rs/zerolog/log"

	"go.opentelemetry.io/otel"
)

// Manages an entire set of containers
// TODO: testing for this module

type ContainerManager struct {
	problemContainers map[string]*ProblemContainer
	pcMut             sync.Mutex
	containers        map[string]string
	containersUpdated bool
	meter             otelmetric.Meter
}

func NewContainerManager() *ContainerManager {
	cm := new(ContainerManager)
	cm.meter = otel.Meter("volpe-framework")
	cm.problemContainers = make(map[string]*ProblemContainer)
	cm.containers = make(map[string]string)
	cm.containersUpdated = false
	return cm
}

func (cm *ContainerManager) HasProblem(problemID string) bool {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	_, ok := cm.problemContainers[problemID]
	return ok
}

func (cm *ContainerManager) AddProblem(problemID string, imagePath string) error {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()
	_, ok := cm.problemContainers[problemID]
	if ok {
		log.Warn().Caller().Msgf("retried creating PC for pID %s, ignoring", problemID)
		// WARN: if supporting updating container, must change cm.containers here
		return errors.New("problemID already has container")
	}

	pc, err := NewProblemContainer(problemID, imagePath)
	if err != nil {
		log.Error().Caller().Msgf("error starting pID %s with image %s: %s", problemID, imagePath, err.Error())
		return err
	}
	cm.problemContainers[problemID] = pc
	cm.containers[pc.containerName] = problemID
	cm.containersUpdated = true
	return nil
}

func (cm *ContainerManager) RemoveProblem(problemID string) error {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	delete(cm.problemContainers, problemID)

	return nil
}

func (cm *ContainerManager) GetSubpopulations() ([]*common.Population, error) {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()
	pops := make([]*common.Population, len(cm.problemContainers))

	var err error = nil

	i := 0
	for k, v := range cm.problemContainers {
		pops[i], err = v.GetSubpopulation()
		if err != nil {
			log.Error().Caller().Msgf("error fetching subpop on %s: %s", k, err.Error())
			return nil, err
		}
		pops[i].ProblemID = &k
		i += 1
	}
	return pops, nil
}

func (cm *ContainerManager) GetSubpopulation(problemID string) (*common.Population, error) {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	container, ok := cm.problemContainers[problemID]
	if !ok {
		log.Error().Caller().Msgf("unknown problemID %s", problemID)
		return nil, errors.New("unknown problemID")
	}
	return container.GetSubpopulation()
}

// func (cm *ContainerManager) HandlePopulationEvents(eventChannel chan *volpe.AdjustPopulationMessage) {
// 	for {
// 		msg, ok := <-eventChannel
// 		if !ok {
// 			log.Error().Caller().Msgf("event channel to CM closed")
// 			return
// 		}
// 		cm.handleEvent(msg)
// 	}
// }

func (cm *ContainerManager) IncorporatePopulation(pop *common.Population) {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	cont, ok := cm.problemContainers[pop.GetProblemID()]
	if !ok {
		log.Error().Caller().Msgf("problemID %s nonexistent for incorp. population", pop.GetProblemID())
		return
	}
	reply, err := cont.commsClient.InitFromSeedPopulation(context.Background(), pop)
	if err != nil {
		log.Error().Caller().Msgf("couldn't incorp popln for problemID %s: %s",
			pop.GetProblemID(),
			err.Error(),
		)
		return
	}
	if !reply.Success {
		log.Error().Caller().Msgf("incorp failed for problem %s: %s",
			pop.GetProblemID(),
			reply.GetMessage(),
		)
		return
	}
}

func (cm *ContainerManager) HandlePopulationEvent(event *volpe.AdjustPopulationMessage) {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()
	pc, ok := cm.problemContainers[event.GetProblemID()]
	if !ok {
		log.Error().Caller().Msgf("received msg for problem ID %s, but problem container does not exist, creation not handled yet", event.GetProblemID())
		// TODO: add logic to create container on worker
		return
	}
	popSize := &ccoms.PopulationSize{Size: event.Size}
	_, err := pc.commsClient.AdjustPopulationSize(context.Background(), popSize)
	if err != nil {
		log.Error().Caller().Msgf("pop size adjust for pid %s got error %s", event.GetProblemID(), err.Error())
		return
	}
	_, err = pc.commsClient.InitFromSeedPopulation(context.Background(), event.GetSeed())
	if err != nil {
		log.Error().Caller().Msgf("pop seed for pid %s got error %s", event.GetProblemID(), err.Error())
		return
	}
}

func (cm *ContainerManager) RegisterResultListener(problemID string, channel chan *common.Population) error {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	pc, ok := cm.problemContainers[problemID]
	if !ok {
		log.Error().Caller().Msgf("unknown problemID %s", problemID)
		return errors.New("Unknown problemID")
	}
	pc.RegisterResultChannel(channel)
	return nil
}

func (cm *ContainerManager) RemoveResultListener(problemID string, channel chan *common.Population) error {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	pc, ok := cm.problemContainers[problemID]
	if !ok {
		log.Error().Caller().Msgf("unknown problemID %s", problemID)
		return errors.New("Unknown problemID")
	}
	pc.DeRegisterResultChannel(channel)
	close(channel)
	return nil
}
