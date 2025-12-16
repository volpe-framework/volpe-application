package scheduler

import (
	vcomms "volpe-framework/comms/volpe"
)

type Scheduler interface {
	Init() error                                  // initialize the population
	FillSchedule(sched Schedule) error            // get a mapping from <worker, problem> to target popln
	AddWorker(name string, cpuCount int32)                        // add a new worker, update population
	RemoveWorker(name string)                     // remove an existing worker, update population
	UpdateMetrics(metrics *vcomms.MetricsMessage) // update the metrics and objective function
	AddProblem(name string)
	RemoveProblem(name string)
}
