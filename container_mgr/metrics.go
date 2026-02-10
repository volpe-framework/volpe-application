package container_mgr

// TODO: Needs to be done on a problemContainer level

/*
import (
	"context"
	"time"

	"github.com/containers/podman/v5/pkg/bindings/containers"
	pmtypes "github.com/containers/podman/v5/pkg/domain/entities/types"

	"github.com/rs/zerolog/log"
)

type ContainerMetrics struct {
	ContainerName string
	ProblemID string
	MemUsageGB float32
	CpuUtilPerc float32
	NetTxBytes uint32
	NetRxBytes uint32
}

func (cm *ContainerManager) getMetricsChannel(conn context.Context) chan pmtypes.ContainerStatsReport {
	cm.pcMut.Lock()
	defer cm.pcMut.Unlock()

	contNames := make([]string, len(cm.containers))
	i := 0
	for k := range cm.containers {
		contNames[i] = k
		i += 1
	}
	options := containers.StatsOptions{}
	statChan, _ := containers.Stats(conn, contNames,
		options.WithAll(false).WithStream(false),
		// Avoiding using streams here since list of container names might change
		// TODO: consider the alternative of one stream per container?
		// TODO: alternatively, get all and use only the ones required
	)
	return statChan
}

func (cm *ContainerManager) StreamContainerMetrics(metricChan chan *ContainerMetrics, ctx context.Context) {
	conn, err := NewPodmanConnection()
	if err != nil {
		log.Err(err).Msgf("Failed to connect to podman for metric stream")
		close(metricChan)
		return
	}

	for {
		err := ctx.Err()
		if err != nil {
			log.Info().Msgf("Stopping container metric stream")
			close(metricChan)
			return
		}

		pmMetrics := <- cm.getMetricsChannel(conn)
		if pmMetrics.Error != nil {
			log.Err(pmMetrics.Error).Msgf("Failed to read container metrics")
			time.Sleep(30*time.Second)
			continue
		}

		for _, pmMetric := range(pmMetrics.Stats) {
			problemID, ok := cm.containers[pmMetric.Name]
			if !ok {
				log.Debug().Msgf("Container %s is unknown, skipping", pmMetric.ContainerID)
				continue
			}
			netTxBytes := uint32(0)
			netRxBytes := uint32(0)
			for _, netStat := range(pmMetric.Network) {
				netTxBytes += uint32(netStat.TxBytes)
				netRxBytes += uint32(netStat.RxBytes)
			}
			metrics := ContainerMetrics{
				ContainerName: pmMetric.ContainerID,
				ProblemID: problemID,
				CpuUtilPerc: float32(pmMetric.AvgCPU),
				MemUsageGB: float32(pmMetric.MemUsage)/(1024*1024*1024),
				NetTxBytes: netTxBytes,
				NetRxBytes: netRxBytes,
			}
			metricChan <- &metrics
		}
		time.Sleep(5*time.Second)
	}
}
*/
