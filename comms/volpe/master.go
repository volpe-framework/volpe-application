package volpe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"volpe-framework/comms/common"
	"volpe-framework/types"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

// TODO: handle stream closing

type MasterComms struct {
	mcs masterCommsServer
	sr  *grpc.Server
	lis net.Listener
}

type ProblemStore interface {
	GetFileName(problemID string) (string, bool)
}

type masterCommsServer struct {
	UnimplementedVolpeMasterServer
	chans_mut  sync.RWMutex
	channs     map[string]chan *MasterMessage
	metricChan chan *DeviceMetricsMessage
	immigChan    chan *MigrationMessage
	sched SchedulerComms
	probStore  ProblemStore
	eventStream chan string
}

type SchedulerComms interface {
	AddWorker(worker types.Worker);
	RemoveWorker(workerID string);
}

func mcsStreamHandlerThread(
	workerID string,
	stream grpc.BidiStreamingServer[WorkerMessage, MasterMessage],
	masterSendChan chan *MasterMessage,
	metricChan chan *DeviceMetricsMessage,
	// popChan chan *common.Population,
	immigChan chan *MigrationMessage,
	eventStream chan string,
) {

	log.Info().Caller().Msgf("workerID %s connected", workerID)

	masterRecvChan := make(chan *WorkerMessage)
	readerContext, closeReader := context.WithCancel(context.Background())

	readerThread := func(ctx context.Context) {
		defer close(masterRecvChan)
		for {
			if ctx.Err() != nil {
				log.Info().Caller().Msgf("closing readerThread for workerID %s", workerID)
				return
			}
			wm, err := stream.Recv()
			if err != nil {
				log.Error().Caller().Msg(err.Error())
				return
			}
			masterRecvChan <- wm
		}
	}
	go readerThread(readerContext)
	defer closeReader()
	for {
		select {
		case result, ok := <-masterRecvChan:
			if !ok {
				// TODO: Notify of stream closure
				log.Warn().Caller().Msgf("workerID %s channel closed", workerID)
				return
			}
			if result.GetMetrics() != nil {
				log.Info().Caller().Msgf("workerID %s received metrics", workerID)
				jsonMsg, _ := json.Marshal(map[string]any{
					"type": "WorkerMetrics",
					"workerID": workerID,
					"cpu": result.GetMetrics().GetCpuUtilPerc(),
					"mem": result.GetMetrics().GetMemUsageGB(),
				})
				eventStream <- string(jsonMsg)

				metricChan <- result.GetMetrics()
			} else if result.GetMigration() != nil {
				mig := result.GetMigration()
				pop := mig.GetPopulation()
				log.Info().Msgf("Received population of size %d for %s from %s", len(pop.GetMembers()), pop.GetProblemID(), mig.GetWorkerID())

				sumFitness := 0.0
				popSize := 0
				for _, ind := range(pop.GetMembers()) {
					sumFitness += float64(ind.GetFitness())
					popSize += 1
				}

				jsonMsg, _ := json.Marshal(map[string]any{
					"type": "ReceivedWorkerPopulation",
					"workerID": workerID,
					"problemID": pop.GetProblemID(),
					"avgFitness": sumFitness/float64(popSize),
					"popSize": popSize,
				})
				eventStream <- string(jsonMsg)

				immigChan <- mig
			} else if result.GetHello() != nil {
				log.Warn().Caller().Msg("got unexpected HelloMsg from stream for " + workerID)
			}
		case result, ok := <-masterSendChan:
			if !ok {
				log.Info().Caller().Msgf("send chan to workerID %s closed, exiting", workerID)
				return
			}
			adjInst :=  result.GetAdjInst()
			emigMsg := result.GetMigration()
			if adjInst != nil {
				jsonMsg, _ := json.Marshal(map[string]any{
					"type": "SetWorkerInstances",
					"problemID": adjInst.GetProblemID(),
					"workerID": workerID,
					"instances": adjInst.GetInstances(),
				})
				eventStream <- string(jsonMsg)
			} else if emigMsg != nil {
				fitnessSum := float32(0.0)
				indCount := 0
				for _, ind := range(emigMsg.GetPopulation().GetMembers()) {
					fitnessSum += float32(ind.GetFitness())
					indCount += 1
				}
				jsonMsg, _ := json.Marshal(map[string]any{
					"type": "SentMasterPopulation",
					"workerID": workerID,
					"containerID": emigMsg.GetContainerID(),
					"problemID": emigMsg.GetPopulation().GetProblemID(),
					"populationSize": indCount,
					"avgFitness": fitnessSum / float32(indCount),
				})
				eventStream <- string(jsonMsg)
			} else {
				log.Error().Msgf("message sent from master was neither adjust instances nor send popln. ")
			}


			err := stream.Send(result)
			if err != nil {
				log.Err(err).Caller().Msgf("")
				// TODO: inform that stream no longer works
				return
			}
		}
	}
}

func initMasterCommsServer(mcs *masterCommsServer, metricChan chan *DeviceMetricsMessage, eventStream chan string) (err error) {
	mcs.channs = make(map[string]chan *MasterMessage)
	mcs.metricChan = metricChan
	mcs.eventStream = eventStream
	return nil
}

func (mcs *masterCommsServer) StartStreams(stream grpc.BidiStreamingServer[WorkerMessage, MasterMessage]) error {
	protoMsg, err := stream.Recv()
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return err
	}
	workerHelloMsg := protoMsg.GetHello()
	if workerHelloMsg == nil {
		log.Error().Caller().Msg("expected WorkerID msg first")
		return errors.New("expected WorkerID msg first")
	}
	workerID := workerHelloMsg.GetWorkerID().GetId()
	log.Info().Msgf("workerID %s connected to master", workerID)
	fmt.Println(workerID)


	masterSendChan := make(chan *MasterMessage)

	mcs.chans_mut.Lock()
	mcs.channs[workerID] = masterSendChan
	mcs.chans_mut.Unlock()

	mcs.sched.AddWorker(types.Worker{
		WorkerID: workerID, 
		CpuCount: workerHelloMsg.GetCpuCount(),
		MemoryGB: workerHelloMsg.GetMemoryGB(),
	})

	jsonMsg, _ := json.Marshal(map[string]any{
		"type": "WorkerJoined",
		"workerID": workerID,
		"cpuCount": workerHelloMsg.GetCpuCount(),
		"mem": workerHelloMsg.GetMemoryGB(),
	})
	mcs.eventStream <- string(jsonMsg)

	mcsStreamHandlerThread(workerID, stream, masterSendChan, mcs.metricChan, mcs.immigChan, mcs.eventStream)

	log.Info().Msgf("workerID %s left", workerID)

	jsonMsg, _ = json.Marshal(map[string]string{
		"type": "WorkerLeft",
		"workerID": workerID,
	})
	mcs.eventStream <- string(jsonMsg)

	mcs.sched.RemoveWorker(workerID)
	return nil
}

func (mcs *masterCommsServer) GetImage(req *ImageRequest, stream grpc.ServerStreamingServer[common.ImageStreamObject]) error {
	problemID := req.GetProblemID()
	fname, ok := mcs.probStore.GetFileName(problemID)
	if !ok {
		log.Error().Caller().Msgf("missing image for PID %s", problemID)
		return nil
	}
	file, err := os.Open(fname)
	if err != nil {
		log.Err(err).Caller().Msgf("failed to get image")
		return nil
	}

	finfo, _ := os.Stat(fname)
	fileSize := int(finfo.Size())
	done := 0

	buf := make([]byte, 512)

	stream.Send(&common.ImageStreamObject{
		Data: &common.ImageStreamObject_Details{
			Details: &common.ImageDetails{
				ProblemID: problemID,
				ImageSizeBytes: int32(fileSize),
			},
		},
	})

	for done < fileSize {
		bytes, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		stream.Send(&common.ImageStreamObject{
			Data: &common.ImageStreamObject_Chunk{
				Chunk: &common.ImageChunk{
					Data: buf,
				},
			},
		})
		done += bytes
	}
	return nil
}

func (mcs *masterCommsServer) mustEmbedUnimplementedVolpeMasterServer() {}

func NewMasterComms(port uint16, metricChan chan *DeviceMetricsMessage, immigChan chan *MigrationMessage, sched SchedulerComms, probStore ProblemStore, eventStream chan string) (*MasterComms, error) {
	mc := new(MasterComms)
	err := initMasterCommsServer(&mc.mcs, metricChan, eventStream)
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return nil, err
	}

	
	sr := grpc.NewServer()
	mc.sr = sr
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return nil, err
	}
	log.Info().Caller().Msgf("master listening on port %d", port)
	
	mc.lis = lis
	mc.mcs.sched = sched
	mc.mcs.immigChan = immigChan
	mc.mcs.metricChan = metricChan
	mc.mcs.probStore = probStore

	RegisterVolpeMasterServer(sr, &mc.mcs)

	return mc, nil
}

func (mc *MasterComms) Serve() error {
	err := mc.sr.Serve(mc.lis)
	if err != nil {
		log.Error().Caller().Msg(err.Error())
	}
	return err
}

func (mc *MasterComms) SendPopulationSize(workerID string, msg *MasterMessage) error {
	mcchan, ok := mc.mcs.channs[workerID]
	if !ok {
		return errors.New("unknown workerID")
	}
	mcchan <- msg
	return nil
}
