package metrics

type WorkerMetrics struct {
	CpuUtil            float32
	MemUsage           float32
	MemTotal           float32
	ApplicationMetrics map[string]*ApplicationMetrics
}

type ApplicationMetrics struct {
	CpuUtil  float32
	MemUsage float32
}


