package scheduler

import (
	"slices"
	"sync"
	vcomms "volpe-framework/comms/volpe"

	//"github.com/rs/zerolog/log"
)

type PrelimWorker struct {
	workerID string
	cpuCount int32
	memory float32
}

type PrelimProblem struct {
	problemID string
	memory float32
	islandCount int32
}

type PrelimScheduler struct {
	problems []string
	workers  []Worker
	removeList []string
	mut sync.Mutex
}


func NewPrelimScheduler() (*PrelimScheduler, error) {
	sched := &PrelimScheduler{}
	sched.problems = make([]string, 0)
	sched.workers = make([]Worker, 0)
	return sched, nil
}

func (ss *PrelimScheduler) Init() error { return nil }

func (ss *PrelimScheduler) AddWorker(workerID string, cpuCount int32) {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	for _, worker := range ss.workers {
		if worker.workerID == workerID {
			return
		}
	}

	ss.workers = append(ss.workers, Worker{workerID, cpuCount})
}

func (ss *PrelimScheduler) UpdateMetrics(metrics *vcomms.MetricsMessage) {
	// TODO: apply metrics update
	//log.Warn().Caller().Msgf("skipping metrics update for static scheduler")
}

func (ss *PrelimScheduler) RemoveWorker(worker string) {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	workerInd := slices.IndexFunc(ss.workers, func (w Worker) bool { return w.workerID == worker } )
	if workerInd == -1 {
		return
	}
	
	ss.workers = slices.Delete(ss.workers, workerInd, workerInd+1)
}

func (ss *PrelimScheduler) FillSchedule(sched Schedule) error {
	sched.Reset()

	ss.mut.Lock()
	defer ss.mut.Unlock()

	for _, p := range ss.problems {
		for _, w := range ss.workers {
			// TODO: adjust default population size?
			sched.Set(w.workerID, p, w.cpuCount) 
		}
	}

	for _, p := range ss.removeList {
		for _, w := range ss.workers {
			sched.Set(w.workerID, p, 0)
		}
	}
	ss.removeList = slices.Delete(ss.removeList, 0, len(ss.removeList))

	return nil
}

func (ss *PrelimScheduler) RemoveProblem(problemID string) {
	ss.mut.Lock()
	defer ss.mut.Unlock()
	index := slices.Index(ss.problems, problemID)
	if index == -1 {
		return
	}
	ss.problems = slices.Delete(ss.problems, index, index+1)
	ss.removeList = append(ss.removeList, problemID)
}

func (ss *PrelimScheduler) AddProblem(problemID string) {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	if slices.Contains(ss.problems, problemID) {
		return
	}
	ss.problems = append(ss.problems, problemID)
}
