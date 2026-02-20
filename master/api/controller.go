package api

import (
	"io"
	"os"
	"strings"
	"time"

	ccomms "volpe-framework/comms/container"
	"volpe-framework/comms/volpe"

	contman "volpe-framework/container_mgr"
	"volpe-framework/model"
	"volpe-framework/scheduler"

	"volpe-framework/types"

	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type VolpeAPI struct {
	probstore *model.ProblemStore
	sched scheduler.Scheduler
	contman *contman.ContainerManager
	eventStream chan string
	eventOutChannels map[chan string]bool
}

func NewVolpeAPI(ps *model.ProblemStore, sched scheduler.Scheduler, contman *contman.ContainerManager, eventStream chan string) (*VolpeAPI, error) {
	api := &VolpeAPI{
		probstore: ps,
		sched: sched,
		contman: contman,
		eventStream: eventStream,
		eventOutChannels: make(map[chan string]bool),
	}
	go api.distributeResults()
	return api, nil
}

func (va *VolpeAPI) RegisterProblem(c *gin.Context) {
	problemID := c.Param("id")
	if problemID == "" {
		c.AbortWithStatusJSON(400, Response{Success: false, Message: "Missing path param id"})
		return
	}

	req := c.Request

	err := req.ParseMultipartForm(32<<20)
	if err != nil {
		c.AbortWithStatus(400)
		log.Err(err).Caller().Send()
	}

	imageHeaders, ok := req.MultipartForm.File["image"]
	if !ok {
		c.AbortWithStatusJSON(400, FailedResponse("missing image field"))
		return
	}

	metaDataStrings, ok := req.MultipartForm.Value["metadata"]
	if !ok {
		c.AbortWithStatusJSON(400, FailedResponse("missing metadata field in MultipartForm"))
		return
	}

	type imageMetaData struct {
		ProblemID string `json:"problemID"`
		Memory float32 `json:"memory"`
		TargetInstances int32 `json:"targetInstances"`
		MigrationFrequency int32 `json:"migrationFrequency"`
		MigrationSize int32 `json:"migrationSize"`
	}

	metaData := imageMetaData{}
	decoder := json.NewDecoder(strings.NewReader(metaDataStrings[0]))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&metaData)
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		c.AbortWithStatusJSON(400, Response{Success: false, Message: "error parsing metadata"})
		return
	}

	if metaData.ProblemID != problemID {
		c.AbortWithStatusJSON(400, FailedResponse("metadata inconsistent with problemID"))
		return
	}
	
	log.Info().Msgf("creating problemID %s", metaDataStrings[0])

	fname := metaData.ProblemID + ".tar"

	srcFile, _ := imageHeaders[0].Open()
	targetFile, _ := os.Create(fname)
	defer targetFile.Close()
	_, err = io.Copy(targetFile, srcFile)
	if err != nil {
		c.AbortWithStatusJSON(500, FailedResponse("image copying failed"))
		log.Error().Caller().Msgf("image file %s: copy failed, %s", fname, err.Error())
		return
	}

	problemID = metaData.ProblemID

	// TODO: proper file names and path for image
	va.probstore.NewProblem(types.Problem{
		ProblemID: problemID,
		MemoryUsage: metaData.Memory,
		IslandCount: metaData.TargetInstances,
		MigrationFrequency: metaData.MigrationFrequency,
		MigrationSize: metaData.MigrationSize,
	})
	va.probstore.RegisterImage(problemID, fname)

	log.Info().Caller().Msgf("registered image %s", problemID)
}

func (va *VolpeAPI) DeleteProblem(c *gin.Context) {
	problemID := c.Param("id")

	// TODO: also remove from problem store

	if va.contman.HasProblem(problemID) {
		va.contman.UntrackProblem(problemID)
		va.sched.RemoveProblem(problemID)
	}
}

func (va *VolpeAPI) StartProblem(c *gin.Context) {
	problemID := c.Param("id")
	if len(problemID) == 0 {
		c.AbortWithStatusJSON(400, "missing path param ID")
		return
	}

	var problem types.Problem
	if va.probstore.GetMetadata(problemID, &problem) == nil {
		log.Err(&contman.UnknownProblemError{ProblemID: problemID}).Msg("")
		c.Status(404)
	}

	va.sched.AddProblem(problem)
	err := va.contman.TrackProblem(problemID)
	if err != nil {
		log.Err(err).Msgf("failed to track problem %s", problemID)
		c.Status(500)
		return
	}
	err = va.contman.HandleInstancesEvent(
		&volpe.AdjustInstancesMessage{
			ProblemID: problemID,
			Instances: 1,
		},
	)
	if err != nil {
		log.Err(err).Msgf("failed to add problem %s", problemID)
		c.Status(500)
	} else {
		jsonMsg, _ := json.Marshal(map[string]any{
			"type": "ProblemStarted",
			"problemID": problemID,
			"islands": problem.IslandCount,
			"mem": problem.MemoryUsage,
		})
		va.eventStream <- string(jsonMsg)

		c.Status(200)
	}
}

func (va *VolpeAPI) distributeResults() {
	for {
		event, ok := <- va.eventStream
		if !ok {
			log.Error().Msgf("eventStream channel was closed while reading")
			return
		}
		for channel, _ := range(va.eventOutChannels) {
			channel <- event
		}
	}
}

func (va *VolpeAPI) StreamResults (c *gin.Context) {

	log.Info().Msg("streaming results")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.Flush()

	log.Info().Msg("set headers")

	problemID := c.Param("id")

	channel := make(chan *ccomms.ResultPopulation)
	err := va.contman.RegisterResultListener(problemID, channel)
	if err != nil {
		log.Error().Caller().Msgf("error listening to pid %s: %s", problemID, err.Error())
	}

	defer va.contman.RemoveResultListener(problemID, channel)

	for {
		pop, ok := <- channel
		if !ok {
			log.Warn().Caller().Msgf("result stream for %s closed", problemID)
			break
		}
		resultPop := ProblemResult{}
		resultPop.Population = make([]Individual, len(pop.GetMembers()))
		resultPop.ProblemID = problemID
		for i, ind := range pop.Members {
			resultPop.Population[i] = Individual{
				Genotype: ind.GetRepresentation(),
				Fitness: ind.GetFitness(),
			}
		}
		c.Writer.WriteString("data: ")
		jsonb, _ := json.Marshal(resultPop)
		_, err = c.Writer.Write(jsonb)
		if err != nil {
			log.Error().Msgf("error writing result for %s: %s", problemID, err)
			break
		}
		c.Writer.WriteString("\n\n")
		c.Writer.Flush()
	
		time.Sleep(5*time.Second)
	}
}

func (va *VolpeAPI) AbortProblem(c *gin.Context) {
	problemID := c.Param("id")

	jsonMsg, _ := json.Marshal(map[string]any{
		"type": "ProblemStopped",
		"problemID": problemID,
	})
	va.eventStream <- string(jsonMsg)

	va.sched.RemoveProblem(problemID)
	va.contman.UntrackProblem(problemID)
	c.Status(200)
}

func (va *VolpeAPI) EventStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.Flush()

	channel := make(chan string, 5)
	va.eventOutChannels[channel] = true

	// TODO: Race condition here?
	defer func() {
		delete(va.eventOutChannels, channel)
		close(channel)
	}()

	for {
		event, ok := <- channel
		if !ok {
			log.Info().Msgf("EventStream channel closed, ending request")
			return
		}
		_, err := c.Writer.WriteString("data: " + event + "\n\n")
		if err != nil {
			log.Err(err).Msgf("EventStream write failed")
			return
		}
		c.Writer.Flush()
	}
}
