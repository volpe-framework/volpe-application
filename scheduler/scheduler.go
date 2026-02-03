package scheduler

import (
	vcomms "volpe-framework/comms/volpe"
	"volpe-framework/types"
)

type Scheduler interface {
	Init() error                                  // initialize the population
	FillSchedule(sched Schedule) error            // get a mapping from <worker, problem> to target popln
	AddWorker(worker types.Worker)                        // add a new worker, update population
	RemoveWorker(name string)                     // remove an existing worker, update population
	UpdateMetrics(metrics *vcomms.DeviceMetricsMessage) // update the metrics and objective function
	AddProblem(problem types.Problem)
	RemoveProblem(name string)
}
