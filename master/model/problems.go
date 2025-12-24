package model

import (
	"sync"
	"volpe-framework/types"
	"errors"

	"github.com/rs/zerolog/log"
)

type ProblemStore struct {
	// TODO: Add persistence
	problems map[string]*ProblemModel
	mut sync.Mutex
}

type ProblemModel struct {
	metadata types.Problem
	problemFile string
}

func NewProblemStore() (*ProblemStore, error) {
	return &ProblemStore{problems: make(map[string]*ProblemModel)},  nil
}

func (ps *ProblemStore) NewProblem(p types.Problem) {
	ps.mut.Lock()
	defer ps.mut.Unlock()

	problem := ProblemModel{ metadata: p, problemFile: "" }
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
	prob.problemFile = fname
	return nil
}

func (ps *ProblemStore) GetMetadata(id string, p *types.Problem) *types.Problem {
	ps.mut.Lock()
	defer ps.mut.Unlock()

	prob, ok := ps.problems[id]
	if !ok {
		return nil
	}

	*p = prob.metadata
	return p
}

func (ps *ProblemStore) GetFileName(id string) (string, bool) {
	ps.mut.Lock()
	defer ps.mut.Unlock()

	prob, ok := ps.problems[id]
	if !ok {
		return "", false
	}
	return prob.problemFile, true
}
