package api

import (
	"io"
	"time"
	"os"
	// "volpe-framework/comms/common"
	ccomms "volpe-framework/comms/container"

	contman "volpe-framework/container_mgr"
	"volpe-framework/master/model"
	"volpe-framework/scheduler"

	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type VolpeAPI struct {
	probstore *model.ProblemStore
	sched scheduler.Scheduler
	contman *contman.ContainerManager
}

func NewVolpeAPI(ps *model.ProblemStore, sched scheduler.Scheduler, contman *contman.ContainerManager) (*VolpeAPI, error) {
	api := &VolpeAPI{
		probstore: ps,
		sched: sched,
		contman: contman,
	}
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
		log.Err(err).Caller().Msg("")
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
	}

	metaData := imageMetaData{}
	err = json.Unmarshal([]byte(metaDataStrings[0]), &metaData)
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
	va.probstore.NewProblem(problemID)
	va.probstore.RegisterImage(problemID, fname)

	log.Info().Caller().Msgf("registered image %s", problemID)
}

func (va *VolpeAPI) StartProblem(c *gin.Context) {
	problemID := c.Param("id")
	if len(problemID) == 0 {
		c.AbortWithStatusJSON(400, "missing path param ID")
		return
	}

	fname := problemID + ".tar"

	va.sched.AddProblem(problemID)
	va.contman.AddProblem(problemID, fname)
	c.Status(200)
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
	problemID := c.Param("problemID")
	va.sched.RemoveProblem(problemID)
	va.contman.RemoveProblem(problemID)
	c.Status(200)
}
