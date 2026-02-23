package volpe

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"sync"
	"volpe-framework/types"
	vc "volpe-framework/comms/volpe"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TODO: handle stream closing

type WorkerComms struct {
	client   vc.VolpeMasterClient
	stream   grpc.BidiStreamingClient[vc.WorkerMessage, vc.MasterMessage]
	cancelFunc context.CancelFunc
	cancelMutex sync.Mutex
	workerID string
	// TODO: include something for population
}

func NewWorkerComms(endpoint string, workerID string, memoryLimit float32, cpuCount int32) (*WorkerComms, error) {
	// TODO: channel or something for population adjust
	wc := new(WorkerComms)
	wc.workerID = workerID
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return nil, err
	}
	wc.client = vc.NewVolpeMasterClient(conn)

	ctx, cancelFunc := context.WithCancel(context.Background())
	wc.cancelFunc = cancelFunc

	wc.stream, err = wc.client.StartStreams(ctx)
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return nil, err
	}

	log.Debug().Msgf("Memory limit: %f GB", memoryLimit)

	err = wc.stream.Send(&vc.WorkerMessage{
		Message: &vc.WorkerMessage_Hello{
			Hello: &vc.WorkerHello{
				WorkerID: &vc.WorkerID{Id: workerID},
				// TODO: use system config 
				CpuCount: int32(runtime.NumCPU()),
				MemoryGB: float32(memoryLimit),
			},
		},
	})
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return nil, err
	}

	log.Info().Caller().Msg("connected to master, streaming")

	return wc, nil
}

func (wc *WorkerComms) CloseCommms() {
	if wc.cancelFunc != nil {
		wc.cancelFunc()
	}
}

func (wc *WorkerComms) HandleStreams(adjInstChannel chan *vc.AdjustInstancesMessage, immigChan chan *vc.MigrationMessage) {
	for {
		msg, err := wc.stream.Recv()
		if err == io.EOF {
			log.Error().Caller().Msg("master stream closed")
			return
		} else if err != nil {
			log.Error().Caller().Msg(err.Error())
			return
		}
		if msg.GetAdjInst() != nil {
			adjPop := msg.GetAdjInst()
			adjInstChannel <- adjPop
		} else if msg.GetMigration() != nil {
		} else {
			log.Warn().Caller().Msg("received unexpected msg, ignoring")
		}
	}
}

func (wc *WorkerComms) SendDeviceMetrics(metrics *vc.DeviceMetricsMessage) error {
	metrics.WorkerID = wc.workerID
	workerMsg := vc.WorkerMessage{Message: &vc.WorkerMessage_Metrics{Metrics: metrics}}
	err := wc.stream.Send(&workerMsg)
	if err != nil {
		log.Error().Caller().Msgf("sending metrics: %s", err.Error())
	}
	return err
}

func (wc *WorkerComms) SendSubPopulation(migrationMsg *vc.MigrationMessage) error {
	migrationMsg.WorkerID = wc.workerID
	workerMsg := vc.WorkerMessage{Message: &vc.WorkerMessage_Migration{Migration: migrationMsg}}
	err := wc.stream.Send(&workerMsg)
	if err != nil {
		log.Err(err).Caller().Msgf("sending subpop for problemID %s containerID %d", migrationMsg.GetPopulation().GetProblemID(), migrationMsg.GetContainerID())
	}
	log.Debug().Msgf("Sent subpopulation of size %d for problem %s from container %d", len(migrationMsg.GetPopulation().GetMembers()), migrationMsg.GetPopulation().GetProblemID(), migrationMsg.GetContainerID())
	return err
}

func (wc *WorkerComms) GetProblemData(problemID string, meta *types.Problem) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	stream, err := wc.client.GetProblemData(ctx, &vc.ProblemRequest{
		ProblemID: problemID,
	})
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return err
	}


	detailsMsg, err := stream.Recv()
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return err
	}
	details := detailsMsg.GetDetails()
	if details == nil {
		log.Error().Caller().Msgf("expected first msg details for pid %s", problemID)
		return errors.New("expected details msg")
	}

	fileSizeBytes := details.GetImageSizeBytes()
	fname := "./" + problemID + ".tar"
	doneBytes := int32(0)

	file, err := os.Create(fname)
	if err != nil {
		log.Error().Caller().Msgf("could not create file %s: %s", fname, err.Error())
		return  err
	}
	defer file.Close()

	for doneBytes < fileSizeBytes {
		recMsg, err := stream.Recv()
		if err != nil {
			log.Error().Caller().Msgf("PID %s: %s", problemID, err.Error())
			return err
		}
		dataMsg := recMsg.GetChunk()
		if dataMsg == nil {
			log.Error().Caller().Msgf("PID %s: expected data msg", problemID)
			return errors.New("expected data msg")
		}
		data := dataMsg.GetData()
		file.Write(data)

		doneBytes += int32(len(data))
	}

	meta.ProblemID = problemID
	meta.ImagePath = fname
	meta.MigrationFrequency = details.MigrationFrequency
	meta.MigrationSize = details.MigrationSize

	return nil
}
