package scheduler

import "strings"

type Schedule map[string]int32

func (s Schedule) Get(workerID string, problemID string) int32 {
	v, ok := s[problemID+"@"+workerID]
	if ok {
		return v
	} else {
		return 0
	}
}

func (s Schedule) Set(workerID string, problemID string, val int32) {
	s[problemID+"@"+workerID] = val
}

func (s Schedule) Apply(applyFunc func (workerID string, problemID string, val int32)) {
	for k, val := range s {
		substrings := strings.Split(k, "@")
		problemID := substrings[0]
		workerID := substrings[1]
		applyFunc(workerID, problemID, val)
	}
}

func (s Schedule) Reset() {
	for k := range s {
		delete(s, k)
	}
}

func (s Schedule) ToDictOfDicts() map[string]map[string]int {
	resultMap := make(map[string]map[string]int)
	s.Apply(func (w string, p string, val int32) {
		if resultMap[w] == nil {
			resultMap[w] = make(map[string]int)
		}
		resultMap[w][p] = int(val)
	})
	return resultMap
}
