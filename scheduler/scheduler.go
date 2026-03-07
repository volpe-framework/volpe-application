package scheduler

import (
	pcomms "volpe-framework/comms/volpe"
	"volpe-framework/types"
)

type Scheduler interface {
	Init() error                                        // initialize the population
	FillSchedule(sched Schedule) error                  // get a mapping from <worker, problem> to target popln
	AddWorker(worker types.Worker)                      // add a new worker, update population
	RemoveWorker(name string)                           // remove an existing worker, update population
	UpdateMetrics(metrics *pcomms.DeviceMetricsMessage) // update the metrics and objective function
	GetWorkers() []Worker
	AddProblem(problem types.Problem)
	RemoveProblem(name string)
	GetInstanceCount(problemID string) int
}

type Worker struct {
	WorkerID      string         `json:"workerID"`
	CpuCount      int            `json:"cpuCount"`
	MemoryGB      float32        `json:"memoryGB"`
	CpuUtilPerc   float32        `json:"cpuUtilPerc"`
	MemoryUsageGB float32        `json:"memoryUsageGB"`
	Schedule      map[string]int `json:"schedule"`
}
