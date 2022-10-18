//go:build bcc
// +build bcc

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package attacher

import (
	"fmt"
	"runtime"
	"strconv"

	"golang.org/x/sys/unix"

	assets "github.com/sustainable-computing-io/kepler/pkg/bpfassets"
	"github.com/sustainable-computing-io/kepler/pkg/config"

	bpf "github.com/iovisor/gobpf/bcc"

	"k8s.io/klog/v2"
)

type perfCounter struct {
	evType   int
	evConfig int
	enabled  bool
}

type BpfModuleTables struct {
	Module    *bpf.Module
	Table     *bpf.Table
	TimeTable *bpf.Table
}

const (
	CPUCycleLable       = config.CPUCycle
	CPUInstructionLabel = config.CPUInstruction
	CacheMissLabel      = config.CacheMiss
)

var (
	Counters = map[string]perfCounter{
		CPUCycleLable:       {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CPU_CYCLES, true},
		CPUInstructionLabel: {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_INSTRUCTIONS, true},
		CacheMissLabel:      {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CACHE_MISSES, true},
	}
	EnableCPUFreq = true
)

func loadModule(objProg []byte, options []string) (m *bpf.Module, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to attach the bpf program: %v", err)
			klog.Infoln(err)
		}
	}()
	m = bpf.NewModule(string(objProg), options)
	// TODO make all entrypoints yaml-declarable
	sched_switch, err := m.LoadTracepoint("sched_switch")
	if err != nil {
		return nil, fmt.Errorf("failed to load sched_switch: %s", err)
	}
	err = m.AttachTracepoint("sched:sched_switch", sched_switch)
	if err != nil {
		return nil, fmt.Errorf("failed to attach sched_switch: %s", err)
	}

	for arrayName, counter := range Counters {
		t := bpf.NewTable(m.TableId(arrayName), m)
		if t == nil {
			return nil, fmt.Errorf("failed to find perf array: %s", arrayName)
		}
		perfErr := openPerfEvent(t, counter.evType, counter.evConfig)
		if perfErr != nil {
			// some hypervisors don't expose perf counters
			klog.Infof("failed to attach perf event %s: %v\n", arrayName, perfErr)
			counter.enabled = false
		}
	}
	return m, err
}

func AttachBPFAssets() (*BpfModuleTables, error) {
	bpfModules := &BpfModuleTables{}
	program := assets.Program
	objProg, err := assets.Asset(program)
	if err != nil {
		return nil, fmt.Errorf("failed to get program %q: %v", program, err)
	}
	options := []string{
		"-DNUM_CPUS=" + strconv.Itoa(runtime.NumCPU()),
		"-DCPU_FREQ",
	}
	if config.EnabledEBPFCgroupID {
		options = append(options, "-DSET_GROUP_ID")
	}
	m, err := loadModule(objProg, options)
	if err != nil {
		klog.Warningf("failed to attach perf module with options %v: %v, Hardware counter related metrics does not exist\n", options, err)
		options = []string{"-DNUM_CPUS=" + strconv.Itoa(runtime.NumCPU())}
		EnableCPUFreq = false
		m, err = loadModule(objProg, options)
		if err != nil {
			klog.Infof("failed to attach perf module with options %v: %v, not able to load eBPF modules\n", options, err)
			// at this time, there is not much we can do with the eBPF module
			return nil, err
		}
	}

	table := bpf.NewTable(m.TableId("processes"), m)
	timeTable := bpf.NewTable(m.TableId("pid_time"), m)

	bpfModules.Module = m
	bpfModules.Table = table
	bpfModules.TimeTable = timeTable

	klog.Infof("Successfully load eBPF module with option: %s", options)

	return bpfModules, nil
}

func DetachBPFModules(bpfModules *BpfModuleTables) {
	closePerfEvent()
	bpfModules.Module.Close()
}

func GetEnabledCounters() []string {
	var metrics []string
	klog.V(5).Infof("hardeware counter metr %t", config.ExposeHardwareCounterMetrics)
	if !config.ExposeHardwareCounterMetrics {
		klog.V(5).Info("hardeware counter metrics not enabled")
		return metrics
	}

	for metric, counter := range Counters {
		if counter.enabled {
			metrics = append(metrics, metric)
		}
	}
	return metrics
}
