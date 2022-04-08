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

	assets "github.com/sustainable-computing-io/kepler/pkg/bpf_assets"

	bpf "github.com/iovisor/gobpf/bcc"
)

type perfCounter struct {
	evType    int
	evConfig  int
	arrayName string
}

type BpfModuleTables struct {
	Module *bpf.Module
	Table  *bpf.Table
}

var (
	counters = []perfCounter{
		{unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CPU_CYCLES, "cpu_cycles"},
		{unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_INSTRUCTIONS, "cpu_instr"},
		{unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CACHE_MISSES, "cache_miss"},
	}
)

func AttachBPFAssets() (*BpfModuleTables, error) {
	bpfModules := &BpfModuleTables{}
	program := assets.Program
	objProg, err := assets.Asset(program)
	if err != nil {
		return nil, fmt.Errorf("failed to get program %q: %v", program, err)
	}
	m := bpf.NewModule(string(objProg), []string{"-DNUM_CPUS=" + strconv.Itoa(runtime.NumCPU())})
	//TODO make all entrypoints yaml-declarable
	sched_switch, err := m.LoadTracepoint("sched_switch")
	if err != nil {
		return nil, fmt.Errorf("failed to load sched_switch: %s", err)
	}
	err = m.AttachTracepoint("sched:sched_switch", sched_switch)
	if err != nil {
		return nil, fmt.Errorf("failed to attach sched_switch: %s", err)
	}

	cpu_freq, err := m.LoadTracepoint("cpu_freq")
	if err != nil {
		return nil, fmt.Errorf("failed to load cpu_freq: %s", err)
	}
	err = m.AttachTracepoint("power:cpu_frequency", cpu_freq)
	if err != nil {
		return nil, fmt.Errorf("failed to attach cpu_frequency: %s", err)
	}

	for _, counter := range counters {
		t := bpf.NewTable(m.TableId(counter.arrayName), m)
		if t == nil {
			return nil, fmt.Errorf("failed to find perf array: %s", counter.arrayName)
		}
		err = openPerfEvent(t, counter.evType, counter.evConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to attach perf event: %s", err)
		}
	}

	table := bpf.NewTable(m.TableId("processes"), m)

	bpfModules.Module = m
	bpfModules.Table = table

	return bpfModules, nil
}

func DetachBPFModules(bpfModules *BpfModuleTables) {
	closePerfEvent()
	bpfModules.Module.Close()
}
