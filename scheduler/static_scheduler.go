package scheduler

import (
	"slices"
	"sync"
	vcomms "volpe-framework/comms/volpe"

	//"github.com/rs/zerolog/log"
)

type StaticScheduler struct {
	problems []string
	workers  []string
	mut sync.Mutex
}


func NewStaticScheduler() (*StaticScheduler, error) {
	sched := &StaticScheduler{}
	sched.problems = make([]string, 0)
	sched.workers = make([]string, 0)
	return sched, nil
}

func (ss *StaticScheduler) Init() error { return nil }

func (ss *StaticScheduler) AddWorker(worker string) {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	if !slices.Contains(ss.workers, worker) {
		ss.workers = append(ss.workers, worker)
	}
}

func (ss *StaticScheduler) UpdateMetrics(metrics *vcomms.MetricsMessage) {
	// TODO: apply metrics update
	//log.Warn().Caller().Msgf("skipping metrics update for static scheduler")
}

func (ss *StaticScheduler) RemoveWorker(worker string) {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	workerInd := slices.Index(ss.workers, worker)
	if workerInd == -1 {
		return
	}
	
	ss.workers = slices.Delete(ss.workers, workerInd, workerInd+1)
}

func (ss *StaticScheduler) FillSchedule(sched Schedule) error {
	sched.Reset()

	ss.mut.Lock()
	defer ss.mut.Unlock()

	for _, p := range ss.problems {
		for _, w := range ss.workers {
			// TODO: adjust default population size?
			sched.Set(w, p, 1000) 
		}
	}
	return nil
}

func (ss *StaticScheduler) RemoveProblem(problemID string) {
	ss.mut.Lock()
	defer ss.mut.Unlock()
	index := slices.Index(ss.problems, problemID)
	if index == -1 {
		return
	}
	ss.problems = slices.Delete(ss.problems, index, index+1)
}

func (ss *StaticScheduler) AddProblem(problemID string) {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	if slices.Contains(ss.problems, problemID) {
		return
	}
	ss.problems = append(ss.problems, problemID)
}
