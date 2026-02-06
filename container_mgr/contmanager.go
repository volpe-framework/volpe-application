package container_mgr

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"slices"
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
	problemContainers 	map[string][]*ProblemContainer // map from problemID to list of containers
	images 				map[string]string				// map from problemID to image path
	pcMut             	sync.Mutex
	containers        	map[string]string				// map from containerID to problemID
	meter             	otelmetric.Meter
	worker 				bool
}

func NewContainerManager(worker bool) *ContainerManager {
	cm := new(ContainerManager)
	cm.meter = otel.Meter("volpe-framework")
	cm.problemContainers = make(map[string][]*ProblemContainer)
	cm.containers = make(map[string]string)
	cm.images = make(map[string]string)
	cm.worker = worker
	return cm
}

func (cm *ContainerManager) HasProblem(problemID string) bool {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	_, ok := cm.problemContainers[problemID]
	return ok
}

func (cm *ContainerManager) GetProblemIDFromContainerName(containerName string) (string, bool) {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	val, ok := cm.containers[containerName]
	return val, ok
}

func (cm *ContainerManager) AddProblem(problemID string, imagePath string) error {
	// Only registers a problem, does not create instances
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	_, ok := cm.problemContainers[problemID]
	cm.images[problemID] = imagePath
	if ok {
		log.Warn().Caller().Msgf("Retried creating PC for pID %s, ignoring", problemID)
		// TODO: if supporting updating container, must change cm.containers here
		return errors.New("problemID already has container")
	}

	instSlice := make([]*ProblemContainer, 0)

	// for inst := 0; inst < instances; inst++ {
	// 	pc, err := NewProblemContainer(problemID, imagePath, cm.worker)
	// 	if err != nil {
	// 		log.Error().Caller().Msgf("error starting pID %s with image %s: %s", problemID, imagePath, err.Error())
	// 		return err
	// 	}
	// 	cm.containers[pc.containerName] = problemID
	// 	instSlice[inst] = pc;
	// }


	cm.problemContainers[problemID] = instSlice
	return nil
}

func (cm *ContainerManager) RemoveProblem(problemID string) error {
	// Stops all containers and removes the problem itself
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	conts, ok := cm.problemContainers[problemID]
	if !ok {
		return nil
	}
	for _, cont := range conts {
		log.Info().Msgf("Stopping container for problem: %s", problemID)
		cont.StopContainer()
	}
	delete(cm.problemContainers, problemID)

	return nil
}

func (cm *ContainerManager) GetSubpopulations(perContainer int) ([]*common.Population, error) {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()
	pops := make([]*common.Population, len(cm.problemContainers))

	i := 0
	for pid, contList := range cm.problemContainers {
		population := common.Population{}
		population.Members = make([]*common.Individual, 0)
		population.ProblemID = &pid
		for _, cont := range contList {
			tmp, err := cont.GetSubpopulation(perContainer)
			if err != nil {
				log.Error().Caller().Msgf("error fetching subpop on %s: %s", pid, err.Error())
				return nil, err
			}

			members := tmp.GetMembers()

			// TODO: allow configuration for this additional logging
			// if cm.worker {
			// 	bestFitness := members[0].GetFitness()
			// 	bestIndex := 0
			// 	for i, memb := range members[1:] {
			// 		fit := memb.GetFitness()
			// 		if fit < bestFitness {
			// 			bestFitness = fit
			// 			bestIndex = i
			// 		}
			// 	}
			// 	fname := cont.containerName+".csv"
			// 	f, err := os.OpenFile(fname, os.O_APPEND | os.O_CREATE | os.O_WRONLY, 0644)
			// 	if err != nil {
			// 		log.Err(err).Msgf("failed while creating/opening file %s to log best", fname)
			// 	} else {
			// 		dataString := fmt.Sprintf("%f,%s\n", bestFitness, base64.RawStdEncoding.EncodeToString(members[bestIndex].GetGenotype()))
			// 		log.Debug().Msgf("Container subpop: %s", dataString)
			// 		_, err := f.WriteString(dataString)
			// 		if err != nil {
			// 			log.Err(err).Msgf("failed to write container pop log")
			// 		}
			// 		f.Close()
			// 	}
			// }

			population.Members = slices.Grow(population.Members, len(members))
			for _, memb := range(members) {
				population.Members = append(population.Members, memb)
			}
		}
		pops[i] = &population
		i += 1
	}
	return pops, nil
}

func (cm *ContainerManager) GetRandomSubpopulation(problemID string, perContainer int) (*common.Population, error) {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	containers, ok := cm.problemContainers[problemID]
	if !ok {
		log.Error().Caller().Msgf("unknown problemID %s", problemID)
		return nil, errors.New("unknown problemID")
	}
	population := new(common.Population)
	population.ProblemID = &problemID

	for _, container := range(containers) {
		subpop, err := container.GetRandomSubpopulation(perContainer)
		if err != nil {
			return nil, err
		}
		members := subpop.GetMembers()
		population.Members = slices.Grow(population.Members, len(members))
		for _, memb := range(members) {
			population.Members = append(population.Members, memb)
		}
	}
	return population, nil
}

func (cm *ContainerManager) IncorporatePopulation(pop *common.Population) {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	containers, ok := cm.problemContainers[pop.GetProblemID()]
	if !ok {
		log.Error().Caller().Msgf("problemID %s nonexistent for incorp. population", pop.GetProblemID())
		return
	}

	perContainer := len(pop.Members)/len(containers)
	for i, cont := range(containers) {
		newpop := common.Population{
			ProblemID: pop.ProblemID, 
			Members: pop.Members[i*perContainer:(i+1)*perContainer],
		}
		reply, err := cont.commsClient.InitFromSeedPopulation(context.Background(), &newpop)
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
}

func (cm *ContainerManager) adjustInstances(containers []*ProblemContainer, problemID string, instances int, seedPop []*common.Individual) ([]*ProblemContainer, error) {
	if len(containers) < int(instances) {
		log.Info().Msgf("Increasing instance count for problem %s to %d", problemID, instances)
		containers = slices.Grow(containers, instances-len(containers))
		for i := len(containers); i < instances; i++ {
			pc, err := NewProblemContainer(problemID, cm.images[problemID], cm.worker)
			if err != nil {
				// log.Error().Caller().Msgf("error starting pID %s with image %s: %s", problemID, cm.images[problemID], err.Error())
				return nil, err
			}
			cm.containers[pc.containerName] = problemID
			containers = append(containers, pc)
		}
		cm.problemContainers[problemID] = containers
	} else if len(containers) > instances {
		log.Info().Msgf("Decreasing instance count for problem %s to %d", problemID, instances)
		for i := instances; i < len(containers);  i++ {
			containers[i].StopContainer()
		}
		cm.problemContainers[problemID] = containers[:instances]
		containers = containers[:instances]
	}
	perContainer := len(seedPop)/len(containers)
	log.Debug().Msgf("Problem: %s Instances: %d SeedPop: %d", problemID, instances, len(seedPop))
	for i, cont := range(containers) {
		newPop := common.Population{
			ProblemID: &problemID,
			Members: seedPop[i*perContainer:(i+1)*perContainer],
		}
		_, err := cont.commsClient.InitFromSeedPopulation(context.Background(), &newPop)
		if err != nil {
			// log.Error().Caller().Msgf("error incorporating popln %s: %s", problemID, err.Error())
			return nil, err
		}
	}
	return containers, nil
}

func (cm *ContainerManager) HandleInstancesEvent(event *volpe.AdjustInstancesMessage) error {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	instances := int(event.GetInstances())
	problemID := event.GetProblemID()

	if instances == 0 {
		cm.RemoveProblem(problemID)
	} else {
		containers, ok := cm.problemContainers[problemID]
		if !ok {
			return fmt.Errorf("Container manager cannt handle instance event for unknown problemID %s", problemID)
		}
		containers, err := cm.adjustInstances(containers, problemID, instances, event.Seed.GetMembers())
		if err != nil {
			return err
		}
		cm.problemContainers[problemID] = containers
	}
	return nil
}

func (cm *ContainerManager) RegisterResultListener(problemID string, channel chan *ccoms.ResultPopulation) error {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	pc, ok := cm.problemContainers[problemID]
	if !ok {
		log.Error().Caller().Msgf("unknown problemID %s", problemID)
		return errors.New("Unknown problemID")
	}
	pc[0].RegisterResultChannel(channel)
	return nil
}

func (cm *ContainerManager) RemoveResultListener(problemID string, channel chan *ccoms.ResultPopulation) error {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	pc, ok := cm.problemContainers[problemID]
	if !ok {
		log.Error().Caller().Msgf("unknown problemID %s", problemID)
		return errors.New("Unknown problemID")
	}
	pc[0].DeRegisterResultChannel(channel)
	close(channel)
	return nil
}
