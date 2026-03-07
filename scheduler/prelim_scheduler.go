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
	problems       []types.Problem
	workers        map[string]*Worker
	removeList     []string
	instanceCounts map[string]int
	mut            sync.Mutex
}

func NewPrelimScheduler() (*PrelimScheduler, error) {
	sched := &PrelimScheduler{}
	sched.problems = make([]types.Problem, 0)
	sched.workers = make(map[string]*Worker)
	sched.instanceCounts = make(map[string]int)
	return sched, nil
}

func (ss *PrelimScheduler) Init() error { return nil }

func (ss *PrelimScheduler) AddWorker(worker types.Worker) {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	ss.workers[worker.WorkerID] = &Worker{
		WorkerID:      worker.WorkerID,
		CpuCount:      int(worker.CpuCount),
		MemoryGB:      worker.MemoryGB,
		Schedule:      make(map[string]int),
		CpuUtilPerc:   0,
		MemoryUsageGB: 0,
	}
}

func (ss *PrelimScheduler) UpdateMetrics(metrics *vcomms.DeviceMetricsMessage) {
	// TODO: apply metrics update
	//log.Warn().Caller().Msgf("skipping metrics update for static scheduler")
	ss.mut.Lock()
	worker := ss.workers[metrics.WorkerID]
	ss.mut.Unlock()

	worker.MemoryUsageGB = metrics.MemUsageGB
	worker.CpuUtilPerc = metrics.CpuUtilPerc
}

func (ss *PrelimScheduler) RemoveWorker(worker string) {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	delete(ss.workers, worker)
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

	for _, p := range ss.problems {
		ss.instanceCounts[p.ProblemID] = 0
	}

	workers := make([]types.Worker, len(ss.workers))
	i := 0
	for _, worker := range ss.workers {
		workers[i] = types.Worker{
			WorkerID: worker.WorkerID,
			CpuCount: int32(worker.CpuCount),
			MemoryGB: worker.MemoryGB,
		}
	}

	slices.SortFunc(workers, func(w1 types.Worker, w2 types.Worker) int {
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
		for _, w := range workers {
			remainingMem := workersRemainingMem[w.WorkerID]
			if workersRemainingMem[w.WorkerID] <= 0 {
				continue
			}
			for _, p := range ss.problems {
				pMem := max(0.5, p.MemoryUsage)
				if pMem == 0.5 {
					log.Warn().Msgf("Invalid memory %f GB for problem %s", p.MemoryUsage, p.ProblemID)
				}
				if remainingMem >= pMem {
					newVal := sched.Get(w.WorkerID, p.ProblemID) + 1
					sched.Set(w.WorkerID, p.ProblemID, newVal)
					ss.workers[w.WorkerID].Schedule[p.ProblemID] = int(newVal)
					ss.instanceCounts[p.ProblemID] += 1
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
			delete(ss.workers[w.WorkerID].Schedule, p)
			log.Info().Msgf("Removing problem %s on scheduler for worker %s", p, w.WorkerID)
		}
	}
	// ss.removeList = make([]string, 0)
	ss.removeList = slices.Delete(ss.removeList, 0, len(ss.removeList))

	return nil
}

func (ss *PrelimScheduler) GetWorkers() []Worker {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	workers := make([]Worker, len(ss.workers))
	i := 0
	for wID, worker := range ss.workers {
		schedCopy := make(map[string]int)
		for k, v := range worker.Schedule {
			schedCopy[k] = v
		}
		workers[i] = Worker{
			WorkerID:      wID,
			CpuCount:      worker.CpuCount,
			MemoryGB:      worker.MemoryGB,
			Schedule:      schedCopy,
			CpuUtilPerc:   worker.CpuUtilPerc,
			MemoryUsageGB: worker.MemoryUsageGB,
		}
	}
	return workers
}

func (ss *PrelimScheduler) RemoveProblem(problemID string) {
	ss.mut.Lock()
	defer ss.mut.Unlock()
	index := slices.IndexFunc(ss.problems, func(x types.Problem) bool { return x.ProblemID == problemID })
	if index == -1 {
		return
	}
	delete(ss.instanceCounts, problemID)
	ss.problems = slices.Delete(ss.problems, index, index+1)
	ss.removeList = append(ss.removeList, problemID)
}

func (ss *PrelimScheduler) GetInstanceCount(problemID string) int {
	ss.mut.Lock()
	defer ss.mut.Unlock()
	return ss.instanceCounts[problemID]
}

func (ss *PrelimScheduler) AddProblem(problem types.Problem) {
	ss.mut.Lock()
	defer ss.mut.Unlock()

	ss.instanceCounts[problem.ProblemID] = 0

	if slices.Contains(ss.problems, problem) {
		return
	}
	ss.problems = append(ss.problems, problem)
}
