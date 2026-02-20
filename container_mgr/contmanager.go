// container manager and functions
package container_mgr

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"sync"

	"volpe-framework/comms/common"
	ccoms "volpe-framework/comms/container"
	"volpe-framework/comms/volpe"
	"volpe-framework/model"
	"volpe-framework/types"

	"github.com/rs/zerolog/log"
)

var ErrUnknownProblem error = errors.New("This problem has no containers")

// Manages an entire set of problem containers
// TODO: testing for this module
type ContainerManager struct {
	problemContainers 	map[string]*cmProblem // map from problemID to list of containers
	pcMut             	sync.Mutex
	// meter             	otelmetric.Meter
	ctx					context.Context
	cancelFunc 			context.CancelFunc
	worker 				bool
	emigChan			chan *volpe.MigrationMessage
	problemStore		*model.ProblemStore
}

// stores all the information related to an individual problem
type cmProblem struct {
	problemContainers 	[]*ProblemContainer
	problemContext 		context.Context
	problemCancel 		context.CancelFunc
}

// constructor for a new container manager
func (cm *ContainerManager) initCM(rootContext context.Context, problemStore *model.ProblemStore) *ContainerManager {
	// cm.meter = otel.Meter("volpe-framework")
	cm.problemContainers = make(map[string]*cmProblem)
	cm.ctx, cm.cancelFunc = context.WithCancel(rootContext)
	cm.problemStore = problemStore
	return cm
}

func NewWorkerContainerManager(rootContext context.Context, problemStore *model.ProblemStore) *ContainerManager {
	cm := new(ContainerManager)
	cm.initCM(rootContext, problemStore)
	cm.worker = true
	cm.emigChan = nil
	return cm
}

func NewMasterContainerManager(rootContext context.Context, problemStore *model.ProblemStore, emigChan chan *volpe.MigrationMessage) *ContainerManager {
	cm := new(ContainerManager)
	cm.initCM(rootContext, problemStore)
	cm.worker = false
	cm.emigChan = emigChan
	return cm
}

// checks if container manager has a specific problem
func (cm *ContainerManager) HasProblem(problemID string) bool {
	cm.lockMut()
	defer cm.unlockMut()

	return cm.hasProblem(problemID)
}

func (cm *ContainerManager) hasProblem(problemID string) bool {
	_, ok := cm.problemContainers[problemID]
	return ok
}

// func (cm *ContainerManager) GetProblemIDFromContainerName(containerName string) (string, bool) {
// 	cm.lockMut()
// 	defer cm.unlockMut()
// 
// 	val, ok := cm.containers[containerName]
// 	return val, ok
// }

func (cm *ContainerManager) lockMut() {
	cm.pcMut.Lock()
	// caller, _, _, ok := runtime.Caller(1)
	// if ok {
	// 	log.Debug().Msgf("Locked pcMut %s", runtime.FuncForPC(caller).Name())
	// }
}

func (cm *ContainerManager) unlockMut() {
	cm.pcMut.Unlock()
	// caller, _, _, ok := runtime.Caller(1)
	// if ok {
	// 	log.Debug().Msgf("Unocked pcMut %s", runtime.FuncForPC(caller).Name())
	// }
}

// prepares the problem to be run
func (cm *ContainerManager) TrackProblem(problemID string) error {
	cm.lockMut()
	defer cm.unlockMut()

	meta := types.Problem{}
	if cm.problemStore.GetMetadata(problemID, &meta) == nil {
		return ErrUnknownProblem
	}

	if cm.hasProblem(problemID) {
		log.Warn().Msgf("problemID %s already tracked in container manager", problemID)
		return nil
	}

	problem := new(cmProblem)
	problem.problemContext, problem.problemCancel = context.WithCancel(cm.ctx)
	problem.problemContainers = make([]*ProblemContainer, 0)
	cm.problemContainers[problemID] = problem

	return nil
}

func (cm *ContainerManager) untrackProblem(problemID string) error {
	log.Debug().Msg("called remove problem")

	problem, ok := cm.problemContainers[problemID]
	if !ok {
		return ErrUnknownProblem
	}
	delete(cm.problemContainers, problemID)

	problem.problemCancel()
	return nil
}

// Stops all containers and untracks the problem
func (cm *ContainerManager) UntrackProblem(problemID string) error {
	cm.lockMut()
	defer cm.unlockMut()
	return cm.untrackProblem(problemID)
}

// THIS TODO: replace with per-container goroutine
// func (cm *ContainerManager) GetSubpopulations(perContainer int) ([]*common.Population, error) {
// 	cm.lockMut()
// 	defer cm.unlockMut()
// 
// 	pops := make([]*common.Population, len(cm.problemContainers))
// 
// 	i := 0
// 	for pid, problem := range cm.problemContainers {
// 		population := common.Population{}
// 		population.Members = make([]*common.Individual, 0)
// 		population.ProblemID = &pid
// 		for _, cont := range problem.problemContainers {
// 			tmp, err := cont.GetSubpopulation(perContainer)
// 			if err != nil {
// 				log.Error().Caller().Msgf("error fetching subpop on %s: %s", pid, err.Error())
// 				return nil, err
// 			}
// 
// 			members := tmp.GetMembers()
// 
// 			// TODO: config for this additional logging
// 			// if cm.worker {
// 			// 	bestFitness := members[0].GetFitness()
// 			// 	bestIndex := 0
// 			// 	for i, memb := range members[1:] {
// 			// 		fit := memb.GetFitness()
// 			// 		if fit < bestFitness {
// 			// 			bestFitness = fit
// 			// 			bestIndex = i
// 			// 		}
// 			// 	}
// 			// 	fname := cont.containerName + ".csv"
// 			// 	f, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
// 			// 	if err != nil {
// 			// 		log.Err(err).Msgf("failed while creating/opening file %s to log best", fname)
// 			// 	} else {
// 			// 		dataString := fmt.Sprintf("%f,%s\n", bestFitness, base64.RawStdEncoding.EncodeToString(members[bestIndex].GetGenotype()))
// 			// 		log.Debug().Msgf("Container subpop: %s", dataString)
// 			// 		_, err := f.WriteString(dataString)
// 			// 		if err != nil {
// 			// 			log.Err(err).Msgf("failed to write container pop log")
// 			// 		}
// 			// 		f.Close()
// 			// 	}
// 			// }
// 
// 			population.Members = slices.Grow(population.Members, len(members))
// 			for _, memb := range members {
// 				population.Members = append(population.Members, memb)
// 			}
// 		}
// 		pops[i] = &population
// 		i += 1
// 	}
// 	return pops, nil
// }

// func (cm *ContainerManager) GetRandomSubpopulation(problemID string, perContainer int) (*common.Population, error) {
// 	cm.lockMut()
// 	defer cm.unlockMut()
// 
// 	problem, ok := cm.problemContainers[problemID]
// 	if !ok {
// 		log.Error().Caller().Msgf("unknown problemID %s", problemID)
// 		return nil, ErrUnknownProblem
// 	}
// 	population := new(common.Population)
// 	population.ProblemID = &problemID
// 
// 	for _, container := range(problem.problemContainers) {
// 		subpop, err := container.GetRandomSubpopulation(perContainer)
// 		if err != nil {
// 			return nil, err
// 		}
// 		members := subpop.GetMembers()
// 		population.Members = slices.Grow(population.Members, len(members))
// 		for _, memb := range members {
// 			population.Members = append(population.Members, memb)
// 		}
// 	}
// 	return population, nil
// }

func (cm *ContainerManager) incPopToPC(pop *common.Population, pc *ProblemContainer) error {
	reply, err := pc.commsClient.InitFromSeedPopulation(context.Background(), pop)
	if err != nil {
		return err
	}
	if !reply.Success {
		return fmt.Errorf("incorp failed for problem %s: %s", pop.GetProblemID(), reply.GetMessage())
	}
	return nil
}

func (cm *ContainerManager) IncorporatePopulation(mig *volpe.MigrationMessage) error {
	cm.lockMut()
	defer cm.unlockMut()

	problem, ok := cm.problemContainers[mig.GetPopulation().GetProblemID()]
	if !ok {
		return ErrUnknownProblem
	}

	pop := mig.GetPopulation()

	if cm.worker {
		// send to the right problemContainer
		containerID := mig.GetContainerID()
		if int(containerID) >= len(problem.problemContainers) {
			log.Warn().Msgf("containerID %d invalid for problemID %s (has %d), sending to random container", containerID, pop.GetProblemID(), len(problem.problemContainers))
			pc := problem.problemContainers[rand.Int() % len(problem.problemContainers)]
			return cm.incPopToPC(pop, pc)
		} else {
			return cm.incPopToPC(pop, problem.problemContainers[containerID])
		}
	} else {
		err :=  cm.incPopToPC(pop, problem.problemContainers[0])
		if err != nil {
			return err
		}
		newPop, err := problem.problemContainers[0].GetRandomSubpopulation()
		if err != nil {
			return err
		}
		// send an emigration msg to the same container
		migMsg := volpe.MigrationMessage{
			Population: newPop,
			WorkerID: mig.GetWorkerID(),
			ContainerID: mig.GetContainerID(),
		}
		cm.emigChan <- &migMsg
	}
	return nil
}

func (cm *ContainerManager) adjustInstances(problemID string, instances int) error {
	problem := cm.problemContainers[problemID]
	containers := problem.problemContainers
	if len(containers) < int(instances) {
		log.Info().Msgf("Increasing instance count for problem %s to %d", problemID, instances)
		containers = slices.Grow(containers, instances-len(containers))
		problemContext := problem.problemContext
		problemMeta := types.Problem{}
		if cm.problemStore.GetMetadata(problemID, &problemMeta) == nil {
			return ErrUnknownProblem
		}
		for i := len(containers); i < instances; i++ {
			pc, err := NewProblemContainer(problemID, &problemMeta, cm.worker, problemContext, cm.emigChan)
			if err != nil {
				return err
			}
			containers = append(containers, pc)
		}
		problem.problemContainers = containers
	} else if len(containers) > instances {
		log.Info().Msgf("Decreasing instance count for problem %s to %d", problemID, instances)
		for i := instances; i < len(containers);  i++ {
			containers[i].Stop()
		}
		problem.problemContainers = containers[:instances]
		containers = containers[:instances]
	}

	// TODO: move to separate function to handle migration
	// perContainer := len(seedPop) / len(containers)
	// log.Debug().Msgf("Problem: %s Instances: %d SeedPop: %d", problemID, instances, len(seedPop))
	// for i, cont := range containers {
	// 	newPop := common.Population{
	// 		ProblemID: &problemID,
	// 		Members:   seedPop[i*perContainer : (i+1)*perContainer],
	// 	}
	// 	n_failures := 0
	// 	for n_failures <= 5 {
	// 		_, err := cont.commsClient.InitFromSeedPopulation(context.Background(), &newPop)
	// 		if err == nil {
	// 			break
	// 		} else {
	// 			log.Warn().Msgf("error initializing container popln %s: %s", problemID, err.Error())
	// 			time.Sleep(5 * time.Second)
	// 			n_failures += 1
	// 		}
	// 	}
	// 	if n_failures >= 5 {
	// 		return errors.New("Failed to initialize population")
	// 	}
	// }
	return nil
}

// adjusts number of running containers for a problem based on event
func (cm *ContainerManager) HandleInstancesEvent(event *volpe.AdjustInstancesMessage) error {
	cm.lockMut()
	defer cm.unlockMut()

	instances := int(event.GetInstances())
	problemID := event.GetProblemID()

	if instances == 0 {
		cm.untrackProblem(problemID)
	} else if !cm.hasProblem(problemID) {
		log.Warn().Msgf("CM is ignoring instance event for unknown problem %s", problemID)
		return ErrUnknownProblem
	} else {
		_, ok := cm.problemContainers[problemID]
		if !ok {
			return ErrUnknownProblem
		}
		err := cm.adjustInstances(problemID, instances)
		if err != nil {
			return err
		}
	}
	return nil
}

// registers result channel for a problem
func (cm *ContainerManager) RegisterResultListener(problemID string, channel chan *ccoms.ResultPopulation) error {
	cm.lockMut()
	defer cm.unlockMut()

	problem, ok := cm.problemContainers[problemID]
	if !ok {
		return ErrUnknownProblem
	}
	if len(problem.problemContainers) < 1 {
		log.Error().Msgf("no containers for problemID %s, can't register result listener", problemID)
		return ErrUnknownProblem
	}
	problem.problemContainers[0].RegisterResultChannel(channel)
	return nil
}

// deregisters result channel
func (cm *ContainerManager) RemoveResultListener(problemID string, channel chan *ccoms.ResultPopulation) error {
	cm.lockMut()
	defer cm.unlockMut()

	problem, ok := cm.problemContainers[problemID]
	if !ok {
		return ErrUnknownProblem
	}
	if len(problem.problemContainers) < 1 {
		log.Error().Msgf("no containers for problemID %s, can't de-register result listener", problemID)
		return ErrUnknownProblem
	}
	problem.problemContainers[0].DeRegisterResultChannel(channel)
	close(channel)
	return nil
}

func (cm *ContainerManager) Destroy() {
	cm.cancelFunc()
}
