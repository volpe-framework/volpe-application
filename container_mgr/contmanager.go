// contai// container manager and functions
package container_mgr

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sync"
	"time"

	"volpe-framework/comms/common"
	ccoms "volpe-framework/comms/container"
	"volpe-framework/comms/volpe"

	"time"

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
}

// stores all the information related to an individual problem
type cmProblem struct {
	problemContainers 	[]*ProblemContainer
	problemContext 		context.Context
	problemCancel 		context.CancelFunc
	image				string
}

// constructor for a new container manager
func NewContainerManager(worker bool, rootContext context.Context) *ContainerManager {
	cm := new(ContainerManager)
	// cm.meter = otel.Meter("volpe-framework")
	cm.problemContainers = make(map[string]*cmProblem)
	// cm.images = make(map[string]string)
	cm.worker = worker
	cm.ctx, cm.cancelFunc = context.WithCancel(rootContext)
	return cm
}

// checks if container manager has a specific problem
func (cm *ContainerManager) HasProblem(problemID string) bool {
	cm.lockMut()
	defer cm.unlockMut()

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

// starts problem container and adds problem to problemContainers
func (cm *ContainerManager) AddProblem(problemID string, imagePath string, instances int) error {
	// Only registers a problem, does not create instances

	cm.lockMut()
	defer cm.unlockMut()

	problem, ok := cm.problemContainers[problemID]
	if ok {
		problem.image = imagePath
		log.Warn().Msgf("container manager is updating existing problemID %s", problemID)
		return nil
	}

	problem = new(cmProblem)
	problem.image = imagePath
	problem.problemContext, problem.problemCancel = context.WithCancel(cm.ctx)
	problem.problemContainers = make([]*ProblemContainer, 0)
	cm.problemContainers[problemID] = problem

	return nil
}

// remove problem container and removes from problemContainers
func (cm *ContainerManager) removeProblem(problemID string) error {
	// Stops all containers and removes the problem itself
	log.Debug().Msg("called remove problem")

	problem, ok := cm.problemContainers[problemID]
	if !ok {
		return ErrUnknownProblem
	}
	delete(cm.problemContainers, problemID)

	problem.problemCancel()
	return nil
}

func (cm *ContainerManager) RemoveProblem(problemID string) error {
	cm.lockMut()
	defer cm.unlockMut()
	return cm.removeProblem(problemID)
}

func (cm *ContainerManager) GetSubpopulations(perContainer int) ([]*common.Population, error) {
	cm.lockMut()
	defer cm.unlockMut()

	pops := make([]*common.Population, len(cm.problemContainers))

	i := 0
	for pid, problem := range cm.problemContainers {
		population := common.Population{}
		population.Members = make([]*common.Individual, 0)
		population.ProblemID = &pid
		for _, cont := range problem.problemContainers {
			tmp, err := cont.GetSubpopulation(perContainer)
			if err != nil {
				log.Error().Caller().Msgf("error fetching subpop on %s: %s", pid, err.Error())
				return nil, err
			}

			members := tmp.GetMembers()

			// TODO: ditch this additional logging
			if cm.worker {
				bestFitness := members[0].GetFitness()
				bestIndex := 0
				for i, memb := range members[1:] {
					fit := memb.GetFitness()
					if fit < bestFitness {
						bestFitness = fit
						bestIndex = i
					}
				}
				fname := cont.containerName + ".csv"
				f, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				if err != nil {
					log.Err(err).Msgf("failed while creating/opening file %s to log best", fname)
				} else {
					dataString := fmt.Sprintf("%f,%s\n", bestFitness, base64.RawStdEncoding.EncodeToString(members[bestIndex].GetGenotype()))
					log.Debug().Msgf("Container subpop: %s", dataString)
					_, err := f.WriteString(dataString)
					if err != nil {
						log.Err(err).Msgf("failed to write container pop log")
					}
					f.Close()
				}
			}

			population.Members = slices.Grow(population.Members, len(members))
			for _, memb := range members {
				population.Members = append(population.Members, memb)
			}
		}
		pops[i] = &population
		i += 1
	}
	return pops, nil
}

func (cm *ContainerManager) GetRandomSubpopulation(problemID string, perContainer int) (*common.Population, error) {
	cm.lockMut()
	defer cm.unlockMut()

	problem, ok := cm.problemContainers[problemID]
	if !ok {
		log.Error().Caller().Msgf("unknown problemID %s", problemID)
		return nil, ErrUnknownProblem
	}
	population := new(common.Population)
	population.ProblemID = &problemID

	for _, container := range(problem.problemContainers) {
		subpop, err := container.GetRandomSubpopulation(perContainer)
		if err != nil {
			return nil, err
		}
		members := subpop.GetMembers()
		population.Members = slices.Grow(population.Members, len(members))
		for _, memb := range members {
			population.Members = append(population.Members, memb)
		}
	}
	return population, nil
}

func (cm *ContainerManager) IncorporatePopulation(pop *common.Population) error {
	cm.lockMut()
	defer cm.unlockMut()

	problem, ok := cm.problemContainers[pop.GetProblemID()]
	if !ok {
		return ErrUnknownProblem
	}

	perContainer := len(pop.Members)/len(problem.problemContainers)
	for i, cont := range(problem.problemContainers) {
		newpop := common.Population{
			ProblemID: pop.ProblemID,
			Members:   pop.Members[i*perContainer : (i+1)*perContainer],
		}
		reply, err := cont.commsClient.InitFromSeedPopulation(context.Background(), &newpop)
		if err != nil {
			return err
		}
		if !reply.Success {
			return fmt.Errorf("incorp failed for problem %s: %s",
				pop.GetProblemID(),
				reply.GetMessage(),
			)
		}
	}
	return nil
}

func (cm *ContainerManager) adjustInstances(problemID string, instances int, seedPop []*common.Individual) error {
	problem := cm.problemContainers[problemID]
	containers := problem.problemContainers
	if len(containers) < int(instances) {
		log.Info().Msgf("Increasing instance count for problem %s to %d", problemID, instances)
		containers = slices.Grow(containers, instances-len(containers))
		problemContext := problem.problemContext
		for i := len(containers); i < instances; i++ {
			pc, err := NewProblemContainer(problemID, problem.image, cm.worker, problemContext)
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
	perContainer := len(seedPop) / len(containers)
	log.Debug().Msgf("Problem: %s Instances: %d SeedPop: %d", problemID, instances, len(seedPop))
	for i, cont := range containers {
		newPop := common.Population{
			ProblemID: &problemID,
			Members:   seedPop[i*perContainer : (i+1)*perContainer],
		}
		n_failures := 0
		for n_failures <= 5 {
			_, err := cont.commsClient.InitFromSeedPopulation(context.Background(), &newPop)
			if err == nil {
				break
			} else {
				log.Warn().Msgf("error initializing container popln %s: %s", problemID, err.Error())
				time.Sleep(5 * time.Second)
				n_failures += 1
			}
		}
		if n_failures >= 5 {
			return errors.New("Failed to initialize population")
		}
	}
	return nil
}

// adjusts number of running conatiners for a problem based on event
func (cm *ContainerManager) HandleInstancesEvent(event *volpe.AdjustInstancesMessage) error {
	cm.lockMut()
	defer cm.unlockMut()

	instances := int(event.GetInstances())
	problemID := event.GetProblemID()

	if instances == 0 {
		cm.removeProblem(problemID)
	} else {
		_, ok := cm.problemContainers[problemID]
		if !ok {
			log.Error().Caller().Msgf("Received msg for problem ID %s, but problem container does not exist, creation not handled yet", event.GetProblemID())
			// TODO: add logic to create container on worker
			return ErrUnknownProblem
		}
		err := cm.adjustInstances(problemID, instances, event.Seed.GetMembers())
		if err != nil {
			return err
		}
	}
	return nil
}

// INFO registers result channel TRUE
func (cm *ContainerManager) RegisterResultListener(problemID string, channel chan *ccoms.ResultPopulation) error {
	cm.lockMut()
	defer cm.unlockMut()

	problem, ok := cm.problemContainers[problemID]
	if !ok {
		log.Error().Caller().Msgf("unknown problemID %s", problemID)
		return ErrUnknownProblem
	}
	if len(problem.problemContainers) < 1 {
		log.Error().Msgf("no containers for problemID %s, can't register result listener", problemID)
		return ErrUnknownProblem
	}
	problem.problemContainers[0].RegisterResultChannel(channel)
	return nil
}

// deregisters resutl channel to FALSE
func (cm *ContainerManager) RemoveResultListener(problemID string, channel chan *ccoms.ResultPopulation) error {
	cm.lockMut()
	defer cm.unlockMut()

	problem, ok := cm.problemContainers[problemID]
	if !ok {
		log.Error().Caller().Msgf("unknown problemID %s", problemID)
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
