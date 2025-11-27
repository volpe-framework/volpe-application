package container_mgr

import (
	"context"
	"time"

	"github.com/containers/podman/v5/pkg/bindings/containers"
	pmtypes "github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"

	"volpe-framework/comms/volpe"
	volpeComms "volpe-framework/comms/volpe"
)

func (cm *ContainerManager) getMetricsChannel(conn context.Context) chan pmtypes.ContainerStatsReport {
	cm.pcMut.Lock()

	defer cm.pcMut.Unlock()
	contNames := make([]string, len(cm.problemContainers))
	i := 0
	for k := range cm.containers {
		contNames[i] = k
		i += 1
	}
	options := containers.StatsOptions{}
	statChan, _ := containers.Stats(conn, contNames,
		options.WithAll(false).WithStream(false),
	)
	return statChan

}

func (cm *ContainerManager) RunMetricsExport(comms *volpe.WorkerComms, deviceName string) error {
	conn, err := NewPodmanConnection()
	if err != nil {
		log.Error().Caller().Msg(err.Error())
		return err
	}

	cpuUtilPerAppln, _ := cm.meter.Float64Gauge("volpe_cpu_util_per_appln",
		otelmetric.WithDescription("CPU Utilization per appln per container"),
	)

	applnMetrics := make(map[string]*volpeComms.ApplicationMetrics)
	metricsMsg := &volpeComms.MetricsMessage{
		CpuUtil:            0,
		MemTotal:           0,
		MemUsage:           0,
		ApplicationMetrics: applnMetrics,
	}
	attribSets := make(map[string]attribute.Set)
	for {
		statChan := cm.getMetricsChannel(conn)
		report := <-statChan
		totalCPU := float32(0)
		totalMem := float32(0)
		for i := range report.Stats {
			contName := report.Stats[i].Name
			log.Info().Caller().Msgf("reporting on %s", contName)
			cm.pcMut.Lock()
			pblmName, ok := cm.containers[contName]
			cm.pcMut.Unlock()
			if !ok {
				pblmName = "<unknown>"
				log.Warn().Caller().Msgf("unknown container %s", contName)
				continue
			}
			attribSet, ok := attribSets[pblmName]
			if !ok {
				attribSet = attribute.NewSet(
					attribute.KeyValue{Key: attribute.Key("host"), Value: attribute.StringValue(deviceName)},
					attribute.KeyValue{Key: attribute.Key("problem"), Value: attribute.StringValue(pblmName)},
				)
			}
			cpuUtilPerAppln.Record(context.Background(), report.Stats[i].CPU,
				otelmetric.WithAttributeSet(attribSet),
			)

			totalCPU += float32(report.Stats[i].CPU)
			totalMem += float32(report.Stats[i].MemUsage)

			appln, ok := applnMetrics[pblmName]
			if !ok {
				appln = &volpe.ApplicationMetrics{}
				applnMetrics[pblmName] = appln
			}
			appln.CpuUtil = float32(report.Stats[i].CPU)
			appln.MemUsage = float32(report.Stats[i].MemUsage)
		}
		metricsMsg.CpuUtil = totalCPU
		metricsMsg.MemUsage = totalMem
		comms.SendMetrics(metricsMsg)
		time.Sleep(15 * time.Second)
	}
}
