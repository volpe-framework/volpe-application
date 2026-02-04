package api

import (
	"fmt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)


func RunAPI(port int, api *VolpeAPI) {
	r := gin.Default()

	r.Use(cors.Default())

	r.POST("/problems/:id", api.RegisterProblem)
	r.DELETE("/problems/:id", api.DeleteProblem)
	r.GET("/problems/:id/results", api.StreamResults)
	r.PUT("/problems/:id/start", api.StartProblem)
	r.Any("/problems/:id/abort", api.AbortProblem)
	r.GET("/eventStream", api.EventStream)

	r.Static("/static", "public/")

	go r.Run(fmt.Sprintf("0.0.0.0:%d", port))
}
