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

package bpf

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/jaypipes/ghw"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

type exporter struct {
	bpfObjects keplerObjects

	schedSwitchLink link.Link
	irqLink         link.Link
	pageWriteLink   link.Link
	pageReadLink    link.Link

	cpuCyclesPerfEvents       []int
	cpuInstructionsPerfEvents []int
	cacheMissPerfEvents       []int

	enabledHardwareCounters sets.Set[string]
	enabledSoftwareCounters sets.Set[string]
}

func NewExporter() (Exporter, error) {
	e := &exporter{
		cpuCyclesPerfEvents:       []int{},
		cpuInstructionsPerfEvents: []int{},
		cacheMissPerfEvents:       []int{},
		enabledHardwareCounters:   sets.New[string](),
		enabledSoftwareCounters:   sets.New[string](),
	}
	err := e.attach()
	if err != nil {
		e.Detach()
	}
	return e, err
}

func (e *exporter) SupportedMetrics() SupportedMetrics {
	return SupportedMetrics{
		HardwareCounters: e.enabledHardwareCounters.Clone(),
		SoftwareCounters: e.enabledSoftwareCounters.Clone(),
	}
}

func (e *exporter) attach() error {
	// Remove resource limits for kernels <5.11.
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("error removing memlock: %v", err)
	}

	// Load eBPF Specs
	specs, err := loadKepler()
	if err != nil {
		return fmt.Errorf("error loading eBPF specs: %v", err)
	}

	// Adjust map sizes to the number of available CPUs
	numCPU := getCPUCores()
	klog.Infof("Number of CPUs: %d", numCPU)
	for _, m := range specs.Maps {
		// Only resize maps that have a MaxEntries of NUM_CPUS constant
		if m.MaxEntries == 128 {
			m.MaxEntries = uint32(numCPU)
		}
	}

	// Set program global variables
	err = specs.RewriteConstants(map[string]interface{}{
		"SAMPLE_RATE": int32(config.BPFSampleRate),
	})
	if err != nil {
		return fmt.Errorf("error rewriting program constants: %v", err)
	}

	// Load the eBPF program(s)
	if err := specs.LoadAndAssign(&e.bpfObjects, nil); err != nil {
		return fmt.Errorf("error loading eBPF objects: %v", err)
	}

	// Attach the eBPF program(s)
	e.schedSwitchLink, err = link.AttachTracing(link.TracingOptions{
		Program:    e.bpfObjects.KeplerSchedSwitchTrace,
		AttachType: ebpf.AttachTraceRawTp,
	})
	if err != nil {
		return fmt.Errorf("error attaching sched_switch tracepoint: %v", err)
	}
	e.enabledSoftwareCounters[config.CPUTime] = struct{}{}

	if config.ExposeIRQCounterMetrics {
		e.irqLink, err = link.AttachTracing(link.TracingOptions{
			Program:    e.bpfObjects.KeplerIrqTrace,
			AttachType: ebpf.AttachTraceRawTp,
		})
		if err != nil {
			return fmt.Errorf("could not attach irq/softirq_entry: %w", err)
		}
		e.enabledSoftwareCounters[config.IRQNetTXLabel] = struct{}{}
		e.enabledSoftwareCounters[config.IRQNetRXLabel] = struct{}{}
		e.enabledSoftwareCounters[config.IRQBlockLabel] = struct{}{}
	}

	group := "writeback"
	name := "writeback_dirty_page"
	if _, err := os.Stat("/sys/kernel/debug/tracing/events/writeback/writeback_dirty_folio"); err == nil {
		name = "writeback_dirty_folio"
	}
	e.pageWriteLink, err = link.Tracepoint(group, name, e.bpfObjects.KeplerWritePageTrace, nil)
	if err != nil {
		klog.Warningf("failed to attach tp/%s/%s: %v. Kepler will not collect page cache write events. This will affect the DRAM power model estimation on VMs.", group, name, err)
	} else {
		e.enabledSoftwareCounters[config.PageCacheHit] = struct{}{}
	}

	e.pageReadLink, err = link.AttachTracing(link.TracingOptions{
		Program:    e.bpfObjects.KeplerReadPageTrace,
		AttachType: ebpf.AttachTraceFEntry,
	})
	if err != nil {
		klog.Warningf("failed to attach fentry/mark_page_accessed: %v. Kepler will not collect page cache read events. This will affect the DRAM power model estimation on VMs.", err)
	} else if !e.enabledSoftwareCounters.Has(config.PageCacheHit) {
		e.enabledSoftwareCounters[config.PageCacheHit] = struct{}{}
	}

	// Return early if hardware counters are not enabled
	if !config.ExposeHardwareCounterMetrics {
		klog.Infof("Hardware counter metrics are disabled")
		return nil
	}

	cleanup := func() error {
		unixClosePerfEvents(e.cpuCyclesPerfEvents)
		e.cpuCyclesPerfEvents = nil
		unixClosePerfEvents(e.cpuInstructionsPerfEvents)
		e.cpuInstructionsPerfEvents = nil
		unixClosePerfEvents(e.cacheMissPerfEvents)
		e.cacheMissPerfEvents = nil
		e.enabledHardwareCounters.Clear()
		klog.Infof("Hardware counter metrics are disabled")
		return nil
	}

	// Create perf events and update each eBPF map
	e.cpuCyclesPerfEvents, err = unixOpenPerfEvent(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CPU_CYCLES, numCPU)
	if err != nil {
		klog.Warning("Failed to open perf event for CPU cycles: ", err)
		return cleanup()
	}
	e.enabledHardwareCounters[config.CPUCycle] = struct{}{}

	e.cpuInstructionsPerfEvents, err = unixOpenPerfEvent(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_INSTRUCTIONS, numCPU)
	if err != nil {
		klog.Warning("Failed to open perf event for CPU instructions: ", err)
		return cleanup()
	}
	e.enabledHardwareCounters[config.CPUInstruction] = struct{}{}

	e.cacheMissPerfEvents, err = unixOpenPerfEvent(unix.PERF_TYPE_HW_CACHE, unix.PERF_COUNT_HW_CACHE_MISSES, numCPU)
	if err != nil {
		klog.Warning("Failed to open perf event for cache misses: ", err)
		return cleanup()
	}
	e.enabledHardwareCounters[config.CacheMiss] = struct{}{}

	for i, fd := range e.cpuCyclesPerfEvents {
		if err := e.bpfObjects.CpuCyclesEventReader.Update(uint32(i), uint32(fd), ebpf.UpdateAny); err != nil {
			klog.Warningf("Failed to update cpu_cycles_event_reader map: %v", err)
			return cleanup()
		}
	}
	for i, fd := range e.cpuInstructionsPerfEvents {
		if err := e.bpfObjects.CpuInstructionsEventReader.Update(uint32(i), uint32(fd), ebpf.UpdateAny); err != nil {
			klog.Warningf("Failed to update cpu_instructions_event_reader map: %v", err)
			return cleanup()
		}
	}
	for i, fd := range e.cacheMissPerfEvents {
		if err := e.bpfObjects.CacheMissEventReader.Update(uint32(i), uint32(fd), ebpf.UpdateAny); err != nil {
			klog.Warningf("Failed to update cache_miss_event_reader map: %v", err)
			return cleanup()
		}
	}

	return nil
}

func (e *exporter) Detach() {
	// Links
	if e.schedSwitchLink != nil {
		e.schedSwitchLink.Close()
		e.schedSwitchLink = nil
	}

	if e.irqLink != nil {
		e.irqLink.Close()
		e.irqLink = nil
	}

	if e.pageWriteLink != nil {
		e.pageWriteLink.Close()
		e.pageWriteLink = nil
	}

	if e.pageReadLink != nil {
		e.pageReadLink.Close()
		e.pageReadLink = nil
	}

	// Perf events
	unixClosePerfEvents(e.cpuCyclesPerfEvents)
	e.cpuCyclesPerfEvents = nil
	unixClosePerfEvents(e.cpuInstructionsPerfEvents)
	e.cpuInstructionsPerfEvents = nil
	unixClosePerfEvents(e.cacheMissPerfEvents)
	e.cacheMissPerfEvents = nil

	// Objects
	e.bpfObjects.Close()
}

func (e *exporter) CollectProcesses() ([]ProcessMetrics, error) {
	start := time.Now()
	// Get the max number of entries in the map
	maxEntries := e.bpfObjects.Processes.MaxEntries()
	total := 0
	deleteKeys := make([]uint32, maxEntries)
	deleteValues := make([]ProcessMetrics, maxEntries)
	var cursor ebpf.MapBatchCursor
	for {
		count, err := e.bpfObjects.Processes.BatchLookupAndDelete(
			&cursor,
			deleteKeys,
			deleteValues,
			&ebpf.BatchOptions{},
		)
		total += count
		if errors.Is(err, ebpf.ErrKeyNotExist) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to batch lookup and delete: %v", err)
		}
	}
	klog.V(5).Infof("collected %d process samples in %v", total, time.Since(start))
	return deleteValues[:total], nil
}

///////////////////////////////////////////////////////////////////////////
// utility functions

func unixOpenPerfEvent(typ, conf, cpuCores int) ([]int, error) {
	sysAttr := &unix.PerfEventAttr{
		Type:   uint32(typ),
		Size:   uint32(unsafe.Sizeof(unix.PerfEventAttr{})),
		Config: uint64(conf),
	}
	fds := []int{}
	for i := 0; i < cpuCores; i++ {
		cloexecFlags := unix.PERF_FLAG_FD_CLOEXEC
		fd, err := unix.PerfEventOpen(sysAttr, -1, i, -1, cloexecFlags)
		if fd < 0 {
			return nil, fmt.Errorf("failed to open bpf perf event on cpu %d: %w", i, err)
		}
		fds = append(fds, fd)
	}
	return fds, nil
}

func unixClosePerfEvents(fds []int) {
	for _, fd := range fds {
		_ = unix.SetNonblock(fd, true)
		unix.Close(fd)
	}
}

func getCPUCores() int {
	cores := runtime.NumCPU()
	if cpu, err := ghw.CPU(); err == nil {
		// we need to get the number of all CPUs,
		// so if /proc/cpuinfo is available, we can get the number of all CPUs
		cores = int(cpu.TotalThreads)
	}
	return cores
}
