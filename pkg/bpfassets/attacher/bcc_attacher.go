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
	Module       *bpf.Module
	Table        *bpf.Table
	CPUFreqTable *bpf.Table
}

const (
	CPUCycleLabel       = config.CPUCycle
	CPURefCycleLabel    = config.CPURefCycle
	CPUInstructionLabel = config.CPUInstruction
	CacheMissLabel      = config.CacheMiss
	bpfPerfArrayPrefix  = "_hc_reader"
)

var (
	Counters = map[string]perfCounter{
		CPUCycleLabel:       {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CPU_CYCLES, true},
		CPURefCycleLabel:    {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_REF_CPU_CYCLES, true},
		CPUInstructionLabel: {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_INSTRUCTIONS, true},
		CacheMissLabel:      {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CACHE_MISSES, true},
	}
	HardwareCountersEnabled = false
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
	ftswitch, err := m.LoadKprobe("kprobe__finish_task_switch")
	if err != nil {
		return nil, fmt.Errorf("failed to load kprobe__finish_task_switch: %s", err)
	}
	err = m.AttachKprobe("finish_task_switch", ftswitch, -1)
	if err != nil {
		err = m.AttachKprobe("finish_task_switch.isra.0", ftswitch, -1)
		if err != nil {
			return nil, fmt.Errorf("failed to attach finish_task_switch: %s", err)
		}
	}

	for arrayName, counter := range Counters {
		bpfPerfArrayName := arrayName + bpfPerfArrayPrefix
		t := bpf.NewTable(m.TableId(bpfPerfArrayName), m)
		if t == nil {
			return nil, fmt.Errorf("failed to find perf array: %s", bpfPerfArrayName)
		}
		perfErr := openPerfEvent(t, counter.evType, counter.evConfig)
		if perfErr != nil {
			// some hypervisors don't expose perf counters
			klog.Infof("failed to attach perf event %s: %v\n", bpfPerfArrayName, perfErr)
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
	}
	if config.EnabledEBPFCgroupID {
		options = append(options, "-DSET_GROUP_ID")
	}
	// TODO: verify if ebpf can run in the VM without hardware counter support, if not, we can disable the HC part and only collect the cpu time
	m, err := loadModule(objProg, options)
	if err != nil {
		klog.Infof("failed to attach perf module with options %v: %v, not able to load eBPF modules\n", options, err)
		return nil, err
	}

	table := bpf.NewTable(m.TableId("processes"), m)
	cpuFreqTable := bpf.NewTable(m.TableId("cpu_freq_array"), m)

	bpfModules.Module = m
	bpfModules.Table = table
	bpfModules.CPUFreqTable = cpuFreqTable

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
