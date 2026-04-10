package api

import (
	"fmt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func RunAPI(port int, api *VolpeAPI) {
	r := gin.Default()

	r.Use(cors.Default())

	r.GET("/problems", api.ListProblems)

	r.DELETE("/problems/:id", api.DeleteProblem)
	r.GET("/problems/:id/results", api.StreamResults)
	r.POST("/problems/:id", api.RegisterProblem)
	r.PUT("/problems/:id/start", api.StartProblem)
	r.GET("/problems/:id", api.GetProblem)
	r.Any("/problems/:id/abort", api.AbortProblem)
	r.GET("/workers", api.GetWorkers)
	r.GET("/workerCount", api.GetWorkerCount)

	r.GET("/eventStream", api.EventStream)

	go r.Run(fmt.Sprintf("0.0.0.0:%d", port))
}
