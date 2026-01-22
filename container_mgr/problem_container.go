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
	hostPort    uint16
	commsClient ccomms.VolpeContainerClient
	resultChannels map[chan *ccomms.ResultPopulation]bool
	rcMut sync.Mutex
	cancel context.CancelFunc
}

func genContainerName(problemID string) string {
	return fmt.Sprintf("volpe_%s_%d", problemID, rand.Int32())
}

const DEFAULT_CONTAINER_PORT uint16 = 8081

func NewProblemContainer(problemID string, imagePath string, worker bool) (*ProblemContainer, error) {
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

	var ctx context.Context
	ctx, pc.cancel = context.WithCancel(context.Background())

	if !worker {
		go pc.sendResults(ctx)
	}

	if worker {
		go pc.runGenerations(ctx)
	}

	return pc, nil
}

func (pc *ProblemContainer) GetContainerName() string {
	return pc.containerName
}

func (pc *ProblemContainer) RegisterResultChannel(channel chan *ccomms.ResultPopulation) {
	pc.rcMut.Lock()
	defer pc.rcMut.Unlock()

	pc.resultChannels[channel] = true
}

func (pc *ProblemContainer) DeRegisterResultChannel(channel chan *ccomms.ResultPopulation) {
	pc.rcMut.Lock()
	defer pc.rcMut.Unlock()

	delete(pc.resultChannels, channel)
}

func (pc *ProblemContainer) GetRandomSubpopulation(count int) (*comms.Population, error) {
	pop, err := pc.commsClient.GetRandom(context.Background(), &ccomms.PopulationSize{Size: int32(count)})
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return nil, err
	}
	pop.ProblemID = &pc.problemID
	return pop, nil
}

func (pc *ProblemContainer) GetSubpopulation(count int) (*comms.Population, error) {
	pop, err := pc.commsClient.GetBestPopulation(context.Background(), &ccomms.PopulationSize{Size: int32(count)})
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return nil, err
	}
	pop.ProblemID = &pc.problemID
	return pop, nil
}

func (pc *ProblemContainer) HandleEvents(eventChannel chan *vcomms.AdjustPopulationMessage) {
	for {
		msg, ok := <-eventChannel
		if !ok {
			log.Warn().Caller().Msgf("event channel for problemID %s was closed", pc.problemID)
			return
		}
		popSize := &ccomms.PopulationSize{Size: msg.GetSize()}
		reply, err := pc.commsClient.AdjustPopulationSize(context.Background(), popSize)
		if err != nil {
			log.Error().Caller().Msg(err.Error() + ", reply: " + reply.GetMessage())
			continue
		}

		pop := &comms.Population{Members: msg.GetSeed().GetMembers()}
		reply, err = pc.commsClient.InitFromSeedPopulation(context.Background(), pop)
		if err != nil {
			log.Error().Caller().Msg(err.Error() + ", reply: " + reply.GetMessage())
			continue
		}
	}
}

func (pc *ProblemContainer) sendResultOnce(ctx context.Context) {
	pc.rcMut.Lock()
	defer pc.rcMut.Unlock()

	result, err := pc.commsClient.GetResults(ctx, &ccomms.PopulationSize{Size: 10})
	if err != nil {
		log.Err(err).Caller().Msgf("fetching best popln for %s failed", pc.problemID)
		return
	}

	for channel, _ := range pc.resultChannels {
		channel <- result
	}
}

func (pc *ProblemContainer) sendResults(ctx context.Context) {
	for {
		time.Sleep(5*time.Second)
		if ctx.Err() != nil {
			break
		}
		pc.sendResultOnce(ctx)
	}
	for channel, _ := range pc.resultChannels {
		close(channel)
	}
}

func (pc *ProblemContainer) runGenerations(ctx context.Context) {
	for {
		// TODO: configure generation run count
		_, err := pc.commsClient.RunForGenerations(ctx, &ccomms.PopulationSize{Size: 3})
		log.Debug().Msgf("Ran 3 generations for container %s", pc.containerName)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			log.Err(err).Caller().Msgf("running gen for %s failed", pc.problemID)
			time.Sleep(5*time.Second)
		}
	}
	log.Info().Caller().Msgf("stopping gen for %s", pc.problemID)
}

func (pc *ProblemContainer) StopContainer() {
	podman, err := NewPodmanConnection()
	if err != nil {
		log.Err(err).Caller().Msgf("container name: %s", pc.containerName)
		return
	}
	pc.cancel()
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
