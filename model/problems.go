package model

import (
	"sync"
	"volpe-framework/types"
	"errors"

	"github.com/rs/zerolog/log"
)

type ProblemStore struct {
	// TODO: Add persistence
	problems map[string]*types.Problem
	mut sync.Mutex
}

func NewProblemStore() (*ProblemStore, error) {
	return &ProblemStore{problems: make(map[string]*types.Problem)},  nil
}

func (ps *ProblemStore) NewProblem(p types.Problem) {
	ps.mut.Lock()
	defer ps.mut.Unlock()

	problem := p
	ps.problems[p.ProblemID] = &problem
}

func (ps *ProblemStore) RegisterImage(id string, fname string) error {
	ps.mut.Lock()
	defer ps.mut.Unlock()

	// TODO: avoid creating problem if missing details

	prob, ok := ps.problems[id]
	if !ok {
		log.Error().Caller().Msgf("tried registering image for nonexistent problem %s", id) 
		return errors.New("unkown problem " + id)
	}
	prob.ImagePath = fname
	return nil
}

func (ps *ProblemStore) GetMetadata(id string, p *types.Problem) *types.Problem {
	ps.mut.Lock()
	defer ps.mut.Unlock()

	prob, ok := ps.problems[id]
	if !ok {
		return nil
	}

	*p = *prob
	return p
}

func (ps *ProblemStore) GetFileName(id string) (string, bool) {
	ps.mut.Lock()
	defer ps.mut.Unlock()

	prob, ok := ps.problems[id]
	if !ok {
		return "", false
	}

	return prob.ImagePath, prob.ImagePath != ""
}

func (ps *ProblemStore) UpdateMemory(id string, memGB float32) {
	ps.mut.Lock()
	defer ps.mut.Unlock()

	prob, ok := ps.problems[id]
	if !ok {
		log.Error().Msgf("tried to update memory for nonexistent problem %s", id)
		return
	}
	prob.MemoryUsage = memGB
}
