package model

import (
	"sync"
	"volpe-framework/comms/common"
)

type ProblemStore struct {
	// TODO: Add persistence
	problems map[string]*Problem
	mut sync.Mutex
}

type Problem struct {
	ProblemID string
	ImageReady bool
	ResultChan chan *common.Population
}

func NewProblemStore() (*ProblemStore, error) {
	return &ProblemStore{problems: make(map[string]*Problem)},  nil
}

func (ps *ProblemStore) NewProblem(id string) {
	problem := Problem {
		ProblemID: id,
		ImageReady: false,
		ResultChan: nil,
	}

	ps.mut.Lock()
	defer ps.mut.Unlock()

	ps.problems[id] = &problem
}

func (ps *ProblemStore) RegisterImage(id string) {
	ps.mut.Lock()
	defer ps.mut.Unlock()

	// TODO: avoid creating problem if missing details

	prob := ps.problems[id]
	if prob != nil {
		prob.ImageReady = true
	} else {
		ps.problems[id] = &Problem{
			ProblemID: id,
			ImageReady: true,
			ResultChan: nil,
		}
	}
}

func (ps *ProblemStore) StartProblem(id string, channel chan *common.Population) {
	ps.mut.Lock()
	defer ps.mut.Unlock()

	prob := ps.problems[id]
	if prob == nil {
		return
	}
	ps.problems[id].ResultChan = channel
}
