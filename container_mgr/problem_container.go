// problem containers and its functions
package container_mgr

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	comms "volpe-framework/comms/common"
	ccomms "volpe-framework/comms/container"
	"volpe-framework/comms/volpe"
	"volpe-framework/types"

	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ProblemContainer struct {
	problemID     string
	containerName string
	//	containerPort uint16
	hostPort       uint16
	commsClient    ccomms.VolpeContainerClient
	resultChannels map[chan *ccomms.ResultPopulation]bool
	wEmigChan chan *volpe.MigrationMessage
	rcMut sync.Mutex
	containerContext context.Context
	cancel context.CancelFunc
	meta *types.Problem
}

// generates random name for every container
func genContainerName(problemID string) string {
	return fmt.Sprintf("volpe_%s_%d", problemID, rand.Int32())
}

const DEFAULT_CONTAINER_PORT uint16 = 8081

// starts problem container, connects it via grpc, and creates context
func NewProblemContainer(problemID string, meta *types.Problem, worker bool, problemContext context.Context, wEmigChan chan *volpe.MigrationMessage) (*ProblemContainer, error) {
	pc := new(ProblemContainer)
	pc.problemID = problemID
	pc.containerName = genContainerName(problemID)
	pc.resultChannels = make(map[chan *ccomms.ResultPopulation]bool)
	pc.meta = meta
	pc.wEmigChan = wEmigChan

	if worker && pc.wEmigChan == nil {
		return nil, errors.New("wEmigChan required for problemContainer")
	}

	hostPort, err := runImage(meta.ImagePath, pc.containerName, DEFAULT_CONTAINER_PORT)
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return nil, err
	}
	pc.hostPort = hostPort

	cc, err := grpc.NewClient(fmt.Sprintf("localhost:%d", int(hostPort)), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return nil, err
	}
	pc.commsClient = ccomms.NewVolpeContainerClient(cc)

	pc.containerContext, pc.cancel = context.WithCancel(problemContext)

	context.AfterFunc(pc.containerContext, pc.stopContainer)

	if !worker {
		go pc.sendResults()
	}

	if worker {
		go pc.runGenerations()
	}

	return pc, nil
}

// returns container name
func (pc *ProblemContainer) GetContainerName() string {
	return pc.containerName
}

// adds /updates channel to resultChannels
func (pc *ProblemContainer) RegisterResultChannel(channel chan *ccomms.ResultPopulation) {
	pc.rcMut.Lock()
	defer pc.rcMut.Unlock()

	pc.resultChannels[channel] = true
}

// removes channel from resultChannels
func (pc *ProblemContainer) DeRegisterResultChannel(channel chan *ccomms.ResultPopulation) {
	pc.rcMut.Lock()
	defer pc.rcMut.Unlock()

	delete(pc.resultChannels, channel)
}

// returns random population from container
func (pc *ProblemContainer) GetRandomSubpopulation() (*comms.Population, error) {
	pop, err := pc.commsClient.GetRandom(pc.containerContext, &ccomms.PopulationSize{Size: int32(pc.meta.MigrationSize)})
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return nil, err
	}
	if len(pop.GetMembers()) != int(pc.meta.MigrationSize) {
		log.Warn().Msgf("problemID %s, requested %d but got %d for random subpop", pc.meta.ProblemID, pc.meta.MigrationSize, len(pop.GetMembers()))
	}
	pop.ProblemID = &pc.problemID
	return pop, nil
}

// returns best population set from container
func (pc *ProblemContainer) GetSubpopulation(count int) (*comms.Population, error) {
	pop, err := pc.commsClient.GetBestPopulation(pc.containerContext, &ccomms.PopulationSize{Size: int32(count)})
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return nil, err
	}
	if len(pop.GetMembers()) != int(pc.meta.MigrationSize) {
		log.Warn().Msgf("problemID %s, requested %d but got %d for best subpop", pc.meta.ProblemID, pc.meta.MigrationSize, len(pop.GetMembers()))
	}
	pop.ProblemID = &pc.problemID
	return pop, nil
}

// fetches result once and sends to corresponding channel from resultChannels
func (pc *ProblemContainer) sendResultOnce() {
	pc.rcMut.Lock()
	defer pc.rcMut.Unlock()

	result, err := pc.commsClient.GetResults(pc.containerContext, &ccomms.PopulationSize{Size: 10})
	if err != nil {
		log.Err(err).Caller().Msgf("fetching best popln for %s failed", pc.problemID)
		return
	}

	for channel := range pc.resultChannels {
		channel <- result
	}
}

// continuosly sends results while containerContext is valid
func (pc *ProblemContainer) sendResults() {
	for {
		time.Sleep(5*time.Second)
		if pc.containerContext.Err() != nil {
			log.Err(pc.containerContext.Err()).Msgf("Stopping sendResults for problem %s", pc.problemID)
			break
		}
		pc.sendResultOnce()
	}
	for channel := range pc.resultChannels {
		close(channel)
	}
}

func (pc *ProblemContainer) runGenerations() {
	for {
		migrationSizeMsg := ccomms.PopulationSize{Size: pc.meta.MigrationSize}
		_, err := pc.commsClient.RunForGenerations(pc.containerContext, &ccomms.PopulationSize{Size: pc.meta.MigrationFrequency})
		if pc.containerContext.Err() != nil {
			break
		}
		if err != nil {
			log.Err(err).Caller().Msgf("running gen for %s failed", pc.problemID)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Debug().Msgf("ProblemID %s ran for %d generations", pc.problemID, pc.meta.MigrationFrequency)
		popln, err := pc.commsClient.GetBestPopulation(pc.containerContext, &migrationSizeMsg)
		if err != nil {
			log.Err(err).Msgf("failed to get best population for %s", pc.problemID)
			break
		}
		popln.ProblemID = &pc.meta.ProblemID
		mig := volpe.MigrationMessage{
			Population: popln,
			WorkerID: "",
			// TODO: set proper containerID
			ContainerID: 0,
		}
		pc.wEmigChan <- &mig
		log.Info().Msgf("Queued emigration population for problem %s", pc.meta.ProblemID)
	}
	log.Info().Caller().Msgf("stopping gen for %s", pc.problemID)
}

// stops and removes container invoking podman
func (pc *ProblemContainer) Stop() {
	pc.cancel()
}

func (pc *ProblemContainer) stopContainer() {
	log.Debug().Msgf("stopping container %s", pc.containerName)
	podman, err := NewPodmanConnection()
	if err != nil {
		log.Err(err).Caller().Msgf("container name: %s", pc.containerName)
		return
	}
	forceRemove := true
	options := containers.RemoveOptions{
		Force: &forceRemove,
	}
	_, err = containers.Remove(podman, pc.containerName, &options)
	if err != nil {
		log.Err(err).Caller().Msgf("err removing container: %s", pc.containerName)
		return
	}
}

