package model

import (
	"sync"
	"volpe-framework/comms/common"
	"errors"
)

type ProblemStore struct {
	// TODO: Add persistence
	problems map[string]*Problem
	mut sync.Mutex
}

type Problem struct {
	ProblemID string
	ProblemFile string
	ResultChan chan *common.Population
}

func NewProblemStore() (*ProblemStore, error) {
	return &ProblemStore{problems: make(map[string]*Problem)},  nil
}

func (ps *ProblemStore) NewProblem(id string) {
	problem := Problem {
		ProblemID: id,
		ProblemFile: "",
		ResultChan: nil,
	}

	ps.mut.Lock()
	defer ps.mut.Unlock()

	ps.problems[id] = &problem
}

func (ps *ProblemStore) RegisterImage(id string, fname string) {
	ps.mut.Lock()
	defer ps.mut.Unlock()

	// TODO: avoid creating problem if missing details

	prob := ps.problems[id]
	if prob != nil {
		prob.ProblemFile = fname
	} else {
		ps.problems[id] = &Problem{
			ProblemID: id,
			ProblemFile: fname,
			ResultChan: nil,
		}
	}
}

func (ps *ProblemStore) StartProblem(id string, channel chan *common.Population) error {
	ps.mut.Lock()
	defer ps.mut.Unlock()

	prob := ps.problems[id]
	if prob == nil || prob.ProblemFile == "" {
		return errors.New("Missing or incomplete problem")
	}
	ps.problems[id].ResultChan = channel
	return nil
}
