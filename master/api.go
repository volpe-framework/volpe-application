package main

import (
	"context"
	"io"
	"os"
	"volpe-framework/comms/api"
	"volpe-framework/comms/common"
	"volpe-framework/master/model"
	"volpe-framework/scheduler"
	contman "volpe-framework/container_mgr"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type VolpeAPI struct {
	probstore *model.ProblemStore
	sched scheduler.Scheduler
	contman *contman.ContainerManager
	api.UnimplementedVolpeAPIServer
}

func NewVolpeAPI(ps *model.ProblemStore, sched scheduler.Scheduler, contman *contman.ContainerManager) (*VolpeAPI, error) {
	api := &VolpeAPI{
		probstore: ps,
		sched: sched,
		contman: contman,
	}
	return api, nil
}

func (va *VolpeAPI) RegisterProblem(
	stream grpc.ClientStreamingServer[common.ImageStreamObject, api.Result],
) error {
	var err error

	data, err := stream.Recv()
	if err != nil {
		return err
	}
	details := data.GetDetails()
	if details == nil {
		stream.SendAndClose(&api.Result{
			Success: false,
			ErrorMessage: "Expected details message first",
		})
		log.Error().Caller().Msgf("expected details first")
		return nil
	}

	problemID := details.GetProblemID()
	// TODO: config image path
	fname := problemID + ".tar"
	file, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer file.Close()

	doneBytes := int32(0)
	totalBytes := details.GetImageSizeBytes()

	for doneBytes < totalBytes {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		img := chunk.GetChunk()
		if img == nil {
			stream.SendAndClose(&api.Result{
				Success: false,
				ErrorMessage: "Expected chunk message",
			})
			log.Error().Caller().Msgf("expected chunk message in problemID %s", problemID)
			return nil
		}
		data := img.GetData()
		doneBytes += int32(len(data))
		file.Write(data)
	}
	if err != nil && err != io.EOF {
		log.Error().Caller().Msgf("reg img %s: %s", problemID, err.Error())
		return err
	}
	va.probstore.NewProblem(problemID)
	va.probstore.RegisterImage(problemID)
	log.Info().Caller().Msgf("registered image %s", problemID)
	stream.SendAndClose(&api.Result{Success: true})
	return nil
}

func (va *VolpeAPI) StartProblem(ctx context.Context, req *api.StartProblemRequest) (*api.Result, error) {
	problemID := req.GetProblemID()
	fname := problemID + ".tar"

	va.sched.AddProblem(problemID)
	va.contman.AddProblem(problemID, fname)
	return &api.Result{Success: true}, nil
}

func (va *VolpeAPI) StreamResults (req *api.StreamResultsRequest, stream grpc.ServerStreamingServer[api.ProblemResult]) error {

	channel := make(chan *common.Population)
	problemID := req.GetProblemID()
	err := va.contman.RegisterResultListener(problemID, channel)
	if err != nil {
		return nil
	}
	defer va.contman.RemoveResultListener(problemID, channel)

	for {
		pop, ok := <- channel
		if !ok {
			log.Warn().Caller().Msgf("result stream for %s closed", problemID)
			break
		}
		err = stream.Send(&api.ProblemResult{
			ProblemID: problemID,
			BestResults: pop,
		})
		if err != nil {
			log.Error().Caller().Msgf(err.Error())
			break
		}
	}
	return nil
}

func (va *VolpeAPI) AbortProblem(ctx context.Context, req *api.AbortProblemRequest) (*api.Result, error) {
	problemID := req.GetProblemID()
	va.sched.RemoveProblem(problemID)
	va.contman.RemoveProblem(problemID)
	return &api.Result{
		Success: true,
		ErrorMessage: "",
	}, nil
}
