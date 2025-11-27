package container_mgr_test

import "testing"
import (
	ccomms "volpe-framework/comms/container"
	cman "volpe-framework/container_mgr"
)

func TestContman(t *testing.T) {
	cm := cman.NewContainerManager(false)
	err := cm.AddProblem("p1", "img.tar")
	if err != nil {
		t.Error(err)
	}
}
