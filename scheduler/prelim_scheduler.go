package scheduler

import (
	"slices"
	"sync"
	vcomms "volpe-framework/comms/volpe"
	"volpe-framework/types"

	"github.com/rs/zerolog/log"
	//"github.com/rs/zerolog/log"
)

type PrelimScheduler struct {
	problems []types.Problem
	workers  []types.Worker
	removeList []string
	mut sync.Mutex
}


func NewPrelimScheduler() (*PrelimScheduler, error) {
	sched := &PrelimScheduler{}
	sched.problems = make([]types.Problem, 0)
	sched.workers = make([]types.Worker, 0)
	return sched, nil
}

func (ss *PrelimScheduler) Init() error { return nil }

func (ss *PrelimScheduler) AddWorker(worker types.Worker) {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	for _, w := range ss.workers {
		if worker.WorkerID == w.WorkerID {
			return
		}
	}

	ss.workers = append(ss.workers, worker)
}

func (ss *PrelimScheduler) UpdateMetrics(metrics *vcomms.MetricsMessage) {
	// TODO: apply metrics update
	//log.Warn().Caller().Msgf("skipping metrics update for static scheduler")
}

func (ss *PrelimScheduler) RemoveWorker(worker string) {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	workerInd := slices.IndexFunc(ss.workers, func (w types.Worker) bool { return w.WorkerID == worker } )
	if workerInd == -1 {
		return
	}
	
	ss.workers = slices.Delete(ss.workers, workerInd, workerInd+1)
}

func (ss *PrelimScheduler) FillSchedule(sched Schedule) error {
	sched.Reset()

	ss.mut.Lock()
	defer ss.mut.Unlock()

	slices.SortFunc(ss.problems, func(p1 types.Problem, p2 types.Problem) int { 
		if p2.MemoryUsage > p1.MemoryUsage {
			return 1
		} else {
			return -1
		}
	})

	slices.SortFunc(ss.workers, func(w1 types.Worker, w2 types.Worker) int { 
		if w1.MemoryGB > w2.MemoryGB {
			return -1
		} else {
			return 1
		}
	})

	// islandsRemaining := make(map[string]int)
	// for _, p := range ss.problems {
	// 	islandsRemaining[p.ProblemID] = int(p.IslandCount)
	// }

	// workersRemainingCPU := make(map[string]int)
	workersRemainingMem := make(map[string]float32)

	for _, w := range ss.workers {
		// workersRemainingCPU[w.WorkerID] = int(w.CpuCount)
		workersRemainingMem[w.WorkerID] = w.MemoryGB
	}

	changeCount := 1
	
	for changeCount > 0 { // doneProblems < len(ss.problems) && doneWorkers < len(ss.workers) {
		changeCount = 0
		for _, w := range ss.workers {
			remainingMem := workersRemainingMem[w.WorkerID]
			if workersRemainingMem[w.WorkerID] <= 0 {
				continue
			}
			for _, p := range ss.problems {
				pMem := max(0.5, p.MemoryUsage)
				if pMem == 0.5 {
					log.Warn().Msgf("Invalid memory for Problem %s", p.ProblemID)
				}
				if remainingMem >= pMem {
					sched.Set(w.WorkerID, p.ProblemID, sched.Get(w.WorkerID, p.ProblemID)+1) 
					remainingMem -= pMem
					changeCount += 1
					// log.Info().Msgf("Added problem %s to worker %s", p.ProblemID, w.WorkerID)
				}
			}
			// log.Info().Msgf("worker %s remaining mem %f", w.WorkerID, remainingMem)
			workersRemainingMem[w.WorkerID] = remainingMem
		}
	}

	for _, p := range ss.removeList {
		for _, w := range ss.workers {
			sched.Set(w.WorkerID, p, 0)
			log.Info().Msgf("Removing problem %s on scheduler for worker %s", p, w.WorkerID)
		}
	}
	// ss.removeList = make([]string, 0)
	ss.removeList = slices.Delete(ss.removeList, 0, len(ss.removeList))

	return nil
}

func (ss *PrelimScheduler) RemoveProblem(problemID string) {
	ss.mut.Lock()
	defer ss.mut.Unlock()
	index := slices.IndexFunc(ss.problems, func (x types.Problem) bool { return x.ProblemID == problemID })
	if index == -1 {
		return
	}
	ss.problems = slices.Delete(ss.problems, index, index+1)
	ss.removeList = append(ss.removeList, problemID)
}

func (ss *PrelimScheduler) AddProblem(problem types.Problem)  {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	if slices.Contains(ss.problems, problem) {
		return
	}
	ss.problems = append(ss.problems, problem)
}
