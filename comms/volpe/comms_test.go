package volpe_test

import (
	"testing"

	ccomms "volpe-framework/comms/common"
	comms "volpe-framework/comms/volpe"
)

// Tests comms between master and worker

type DummySched struct {
}

func (ds *DummySched) AddWorker(_ string, _ int32) {
}

func (ds *DummySched) RemoveWorker(_ string) {
}

func TestComms(t *testing.T) {
	// 1. setup master

	metricChan := make(chan *comms.MetricsMessage, 5)
	popChan := make(chan *ccomms.Population, 5)

	dummySched := DummySched{}

	mc, err := comms.NewMasterComms(8118, metricChan, popChan, &dummySched)
	if err != nil {
		t.Fatal(err)
	}
	go mc.Serve()

	// 2. setup worker
	wc, err := comms.NewWorkerComms("localhost:8118", "abcd123")
	if err != nil {
		t.Fatal(err)
	}

	// 3. send metrics on worker, check if received by master
	metricsMsg := comms.MetricsMessage{
		CpuUtil:  1.5,
		MemUsage: 1.5,
		MemTotal: 1.5,
		ApplicationMetrics: map[string]*comms.ApplicationMetrics{
			"application1": {
				CpuUtil:  1.5,
				MemUsage: 1.0,
			},
		},
		WorkerID: "abcd123",
	}
	t.Log("sending metrics to master")
	err = wc.SendMetrics(&metricsMsg)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("sent metrics to master")

	problemID := "application1"

	popMsg := ccomms.Population{
		Members: []*ccomms.Individual{
			&ccomms.Individual{Genotype: []byte{1, 2, 3}, Fitness: 10.0},
		},
		ProblemID: &problemID,
	}
	err = wc.SendSubPopulation(&popMsg)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("sent pop to master")

	recMsg, ok := <-metricChan
	if !ok {
		t.Fatal("channel closed")
	}
	if recMsg.CpuUtil != 1.5 || recMsg.MemUsage != 1.5 || recMsg.WorkerID != "abcd123" {
		t.Fail()
	}
	t.Log("metrics received properly")

	recPop, ok := <-popChan
	if !ok {
		t.Fatal("channel closed")
	}
	if len(recPop.Members) != 1 || *recPop.ProblemID != problemID {
		t.Fail()
	}
	t.Log("pop received properly")
}
