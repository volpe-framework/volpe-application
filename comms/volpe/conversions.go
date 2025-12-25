package volpe

import (
	"volpe-framework/types"
)

func WorkerMetricsToProtobuf(wm *types.WorkerMetrics) MetricsMessage {
	amMap := make(map[string]*ApplicationMetrics)
	for key, val := range wm.ApplicationMetrics {
		amMap[key] = ApplicationMetricsToProtobuf(val)
	}
	return MetricsMessage{
		CpuUtil:            wm.CpuUtil,
		MemUsage:           wm.MemUsage,
		MemTotal:           wm.MemTotal,
		ApplicationMetrics: amMap,
	}
}

func  ApplicationMetricsToProtobuf (am *types.ApplicationMetrics) *ApplicationMetrics {
	res := ApplicationMetrics{
		CpuUtil:  am.CpuUtil,
		MemUsage: am.MemUsage,
	}
	return &res
}
