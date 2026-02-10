// problem containers and its functions
package container_mgr

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	comms "volpe-framework/comms/common"
	ccomms "volpe-framework/comms/container"
	vcomms "volpe-framework/comms/volpe"

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
	rcMut sync.Mutex
	containerContext context.Context
	cancel context.CancelFunc
}

// generates random name for every container
func genContainerName(problemID string) string {
	return fmt.Sprintf("volpe_%s_%d", problemID, rand.Int32())
}

const DEFAULT_CONTAINER_PORT uint16 = 8081

// starts problem container, connects it via grpc, and creates context
func NewProblemContainer(problemID string, imagePath string, worker bool, problemContext context.Context) (*ProblemContainer, error) {
	pc := new(ProblemContainer)
	pc.problemID = problemID
	pc.containerName = genContainerName(problemID)
	pc.resultChannels = make(map[chan *ccomms.ResultPopulation]bool)

	hostPort, err := runImage(imagePath, pc.containerName, DEFAULT_CONTAINER_PORT)
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
func (pc *ProblemContainer) GetRandomSubpopulation(count int) (*comms.Population, error) {
	pop, err := pc.commsClient.GetRandom(pc.containerContext, &ccomms.PopulationSize{Size: int32(count)})
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return nil, err
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
	pop.ProblemID = &pc.problemID
	return pop, nil
}

// handles population updation events
func (pc *ProblemContainer) HandleEvents(eventChannel chan *vcomms.AdjustPopulationMessage) {
	done := pc.containerContext.Done()
	for {
		select {
		case _ = <- done:
			err := pc.containerContext.Err()
			if err != nil {
				log.Err(err).Msgf("exiting handleEvents for problem %s", pc.problemID)
				return
			}
		case msg, ok := <- eventChannel:
			if !ok {
				log.Warn().Caller().Msgf("event channel for problemID %s was closed", pc.problemID)
				return
			}
			popSize := &ccomms.PopulationSize{Size: msg.GetSize()}
			reply, err := pc.commsClient.AdjustPopulationSize(pc.containerContext, popSize)
			if err != nil {
				log.Error().Caller().Msg(err.Error() + ", reply: " + reply.GetMessage())
				continue
			}

			pop := &comms.Population{Members: msg.GetSeed().GetMembers()}
			reply, err = pc.commsClient.InitFromSeedPopulation(pc.containerContext, pop)
			if err != nil {
				log.Error().Caller().Msg(err.Error() + ", reply: " + reply.GetMessage())
				continue
			}

		}
	}
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
		// TODO: configure generation run count
		_, err := pc.commsClient.RunForGenerations(pc.containerContext, &ccomms.PopulationSize{Size: 3})
		if pc.containerContext.Err() != nil {
			break
		}
		if err != nil {
			log.Err(err).Caller().Msgf("running gen for %s failed", pc.problemID)
			time.Sleep(5 * time.Second)
		}
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

