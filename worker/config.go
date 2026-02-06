package main

import (
	ini "gopkg.in/ini.v1"
)

type WorkerConfig struct {
	GeneralConfig GeneralConfig `ini:"general"`
	ResourceConfig ResourceConfig `ini:"resources"`
}

type GeneralConfig struct {
	VolPEMaster string `ini:"volpe_master"`
	WorkerID string `ini:"worker_id"`
}

type ResourceConfig struct {
	MemoryGB float32 `ini:"memory_gb"`
	CpuCount int32 `ini:"cpu_count"`
}

func LoadConfig(configPath string) (*WorkerConfig, error) {
	wc := new(WorkerConfig)
	err := ini.MapTo(wc, configPath)
	if err != nil {
		return nil, err
	} else {
		return wc, nil
	}
}

