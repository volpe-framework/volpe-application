package types

type Problem struct {
	ProblemID string
	MemoryUsage float32
	IslandCount int32
}

type Worker struct {
	WorkerID string
	MemoryGB float32
	CpuCount int32
}
