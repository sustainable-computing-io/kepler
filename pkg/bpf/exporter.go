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
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/jaypipes/ghw"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

type exporter struct {
	bpfObjects      keplerObjects
	cpus            int
	schedSwitchLink link.Link
	irqLink         link.Link
	pageWriteLink   link.Link
	pageReadLink    link.Link
	processFreeLink link.Link

	perfEvents *hardwarePerfEvents

	enabledHardwareCounters sets.Set[string]
	enabledSoftwareCounters sets.Set[string]

	// Locks processMetrics and freedPIDs.
	// Acquired in CollectProcesses - to prevent new events from being processed
	// while summarizing the metrics and resetting the counters.
	// Acquired in handleEvents - to prevent CollectProcesses from summarizing
	// the metrics while we're handling an event from the ring buffer.
	// Note: Release this lock as soon as possible as it will block the
	// processing of new events from the ring buffer.
	mu             *sync.Mutex
	processMetrics map[uint32]*bpfMetrics
	freedPIDs      []int
}

func NewExporter() (Exporter, error) {
	e := &exporter{
		cpus:                    ebpf.MustPossibleCPU(),
		enabledHardwareCounters: sets.New[string](),
		enabledSoftwareCounters: sets.New[string](),
		mu:                      &sync.Mutex{},
		processMetrics:          make(map[uint32]*bpfMetrics),
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

	// Load the eBPF program(s)
	if err := specs.LoadAndAssign(&e.bpfObjects, nil); err != nil {
		return fmt.Errorf("error loading eBPF objects: %v", err)
	}

	// Attach the eBPF program(s)
	e.processFreeLink, err = link.AttachTracing(link.TracingOptions{
		Program:    e.bpfObjects.KeplerSchedProcessFree,
		AttachType: ebpf.AttachTraceRawTp,
	})
	if err != nil {
		return fmt.Errorf("error attaching sched_process_free tracepoint: %v", err)
	}

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

	e.perfEvents, err = createHardwarePerfEvents(
		e.bpfObjects.CpuInstructionsEventReader,
		e.bpfObjects.CpuCyclesEventReader,
		e.bpfObjects.CacheMissEventReader,
		numCPU,
	)
	if err != nil {
		return nil
	}
	e.enabledHardwareCounters[config.CPUCycle] = struct{}{}
	e.enabledHardwareCounters[config.CPUInstruction] = struct{}{}
	e.enabledHardwareCounters[config.CacheMiss] = struct{}{}

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
	if e.perfEvents != nil {
		e.perfEvents.close()
		e.perfEvents = nil
	}

	// Objects
	e.bpfObjects.Close()
}

func (e *exporter) Start(stopChan <-chan struct{}) error {
	rd, err := ringbuf.NewReader(e.bpfObjects.Rb)
	if err != nil {
		return fmt.Errorf("failed to create ring buffer reader: %w", err)
	}
	defer rd.Close()

	for {
		var record *ringbuf.Record

		select {
		case <-stopChan:
			if err := rd.Close(); err != nil {
				return fmt.Errorf("closing ring buffer reader: %w", err)
			}
			return nil
		default:
			var event keplerEvent
			record = new(ringbuf.Record)

			err := rd.ReadInto(record)
			if err != nil {
				if errors.Is(err, ringbuf.ErrClosed) {
					return nil
				}
				if errors.Is(err, ringbuf.ErrFlushed) {
					record.RawSample = record.RawSample[:0]
				}
				klog.Errorf("reading from reader: %s", err)
				continue
			}

			if err := binary.Read(bytes.NewBuffer(record.RawSample), binary.NativeEndian, &event); err != nil {
				klog.Errorf("parsing ringbuf event: %s", err)
				continue
			}

			// Process the event
			e.handleEvent(event)
		}
	}
}

type bpfMetrics struct {
	CGroupID        uint64
	CPUCyles        PerCPUCounter
	CPUInstructions PerCPUCounter
	CacheMiss       PerCPUCounter
	CPUTime         PerCPUCounter
	TxIRQ           uint64
	RxIRQ           uint64
	BlockIRQ        uint64
	PageCacheHit    uint64
}

func (p *bpfMetrics) Reset() {
	p.CPUCyles.Reset()
	p.CPUInstructions.Reset()
	p.CacheMiss.Reset()
	p.CPUTime.Reset()
	p.TxIRQ = 0
	p.RxIRQ = 0
	p.BlockIRQ = 0
	p.PageCacheHit = 0
}

func newBpfMetrics() *bpfMetrics {
	return &bpfMetrics{
		CPUCyles:        NewPerCPUCounter(),
		CPUInstructions: NewPerCPUCounter(),
		CacheMiss:       NewPerCPUCounter(),
		CPUTime:         NewPerCPUCounter(),
	}
}

type PerCPUCounter struct {
	Values map[uint64]uint64
	Total  uint64
}

func NewPerCPUCounter() PerCPUCounter {
	return PerCPUCounter{
		Values: make(map[uint64]uint64),
	}
}

func (p *PerCPUCounter) Start(cpu, taskID uint32, value uint64) {
	key := uint64(cpu)<<32 | uint64(taskID)

	// TODO: The eBPF code would blindly overwrite the value if it already exists.
	// We will preserve the old behavior for now, but we should consider
	// returning an error if the value already exists.
	p.Values[key] = value
}

func (p *PerCPUCounter) Stop(cpu, taskID uint32, value uint64) {
	if value == 0 {
		return
	}

	key := uint64(cpu)<<32 | uint64(taskID)

	if _, ok := p.Values[key]; !ok {
		return
	}

	delta := uint64(0)

	// Probably a clock issue where the recorded on-CPU event had a
	// timestamp later than the recorded off-CPU event, or vice versa.
	if value > p.Values[key] {
		delta = value - p.Values[key]
	}

	p.Total += delta

	delete(p.Values, key)
}

func (p *PerCPUCounter) Reset() {
	// Leave values in place since we may have in-flight
	p.Total = 0
}

func (e *exporter) handleEvent(event keplerEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var p *bpfMetrics

	if _, ok := e.processMetrics[event.Pid]; !ok {
		e.processMetrics[event.Pid] = newBpfMetrics()
	}
	p = e.processMetrics[event.Pid]

	switch event.EventType {
	case uint64(keplerEventTypeSCHED_SWITCH):
		// Handle the new task going on CPU
		p.CPUCyles.Start(event.CpuId, event.Tid, event.CpuCycles)
		p.CPUInstructions.Start(event.CpuId, event.Tid, event.CpuInstr)
		p.CacheMiss.Start(event.CpuId, event.Tid, event.CacheMiss)
		p.CPUTime.Start(event.CpuId, event.Tid, event.Ts)

		// Handle the task going OFF CPU
		if _, ok := e.processMetrics[event.OffcpuPid]; !ok {
			e.processMetrics[event.OffcpuPid] = newBpfMetrics()
		}
		offcpu := e.processMetrics[event.OffcpuPid]
		offcpu.CPUCyles.Stop(event.CpuId, event.OffcpuTid, event.CpuCycles)
		offcpu.CPUInstructions.Stop(event.CpuId, event.OffcpuTid, event.CpuInstr)
		offcpu.CacheMiss.Stop(event.CpuId, event.OffcpuTid, event.CacheMiss)
		offcpu.CPUTime.Stop(event.CpuId, event.OffcpuTid, event.Ts)
		offcpu.CGroupID = event.OffcpuCgroupId
	case uint64(keplerEventTypePAGE_CACHE_HIT):
		p.PageCacheHit += 1
	case uint64(keplerEventTypeIRQ):
		switch event.IrqNumber {
		case uint32(keplerIrqTypeNET_TX):
			p.TxIRQ += 1
		case uint32(keplerIrqTypeNET_RX):
			p.RxIRQ += 1
		case uint32(keplerIrqTypeBLOCK):
			p.BlockIRQ += 1
		}
		return
	case uint64(keplerEventTypeFREE):
		e.freedPIDs = append(e.freedPIDs, int(event.Pid))
	}
}

func (e *exporter) CollectProcesses() (ProcessMetricsCollection, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := ProcessMetricsCollection{
		Metrics:   make([]ProcessMetrics, len(e.processMetrics)),
		FreedPIDs: e.freedPIDs,
	}
	for pid, m := range e.processMetrics {
		result.Metrics = append(result.Metrics, ProcessMetrics{
			CGroupID:        m.CGroupID,
			Pid:             uint64(pid),
			ProcessRunTime:  m.CPUTime.Total / 1000, // convert nanoseconds to milliseconds
			CPUCyles:        m.CPUCyles.Total,
			CPUInstructions: m.CPUInstructions.Total,
			CacheMiss:       m.CacheMiss.Total,
			PageCacheHit:    m.PageCacheHit,
			NetTxIRQ:        m.TxIRQ,
			NetRxIRQ:        m.RxIRQ,
			NetBlockIRQ:     m.BlockIRQ,
		})
		m.Reset()
	}
	// Clear the cache of any PIDs freed this sample period
	e.freedPIDs = []int{}

	return result, nil
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

type hardwarePerfEvents struct {
	cpuCyclesPerfEvents       []int
	cpuInstructionsPerfEvents []int
	cacheMissPerfEvents       []int
}

func (h *hardwarePerfEvents) close() {
	unixClosePerfEvents(h.cpuCyclesPerfEvents)
	unixClosePerfEvents(h.cpuInstructionsPerfEvents)
	unixClosePerfEvents(h.cacheMissPerfEvents)
}

// CreateHardwarePerfEvents creates perf events for CPU cycles, CPU instructions, and cache misses
// and updates the corresponding eBPF maps.
func createHardwarePerfEvents(cpuInstructionsMap, cpuCyclesMap, cacheMissMap *ebpf.Map, numCPU int) (*hardwarePerfEvents, error) {
	var err error
	events := &hardwarePerfEvents{
		cpuCyclesPerfEvents:       make([]int, 0, numCPU),
		cpuInstructionsPerfEvents: make([]int, 0, numCPU),
		cacheMissPerfEvents:       make([]int, 0, numCPU),
	}
	defer func() {
		if err != nil && events != nil {
			unixClosePerfEvents(events.cpuCyclesPerfEvents)
			unixClosePerfEvents(events.cpuInstructionsPerfEvents)
			unixClosePerfEvents(events.cacheMissPerfEvents)
		}
	}()

	// Create perf events and update each eBPF map
	events.cpuCyclesPerfEvents, err = unixOpenPerfEvent(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CPU_CYCLES, numCPU)
	if err != nil {
		klog.Warning("Failed to open perf event for CPU cycles: ", err)
		return nil, err
	}

	events.cpuInstructionsPerfEvents, err = unixOpenPerfEvent(unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_INSTRUCTIONS, numCPU)
	if err != nil {
		klog.Warning("Failed to open perf event for CPU instructions: ", err)
		return nil, err
	}

	events.cacheMissPerfEvents, err = unixOpenPerfEvent(unix.PERF_TYPE_HW_CACHE, unix.PERF_COUNT_HW_CACHE_MISSES, numCPU)
	if err != nil {
		klog.Warning("Failed to open perf event for cache misses: ", err)
		return nil, err
	}

	for i, fd := range events.cpuCyclesPerfEvents {
		if err = cpuCyclesMap.Update(uint32(i), uint32(fd), ebpf.UpdateAny); err != nil {
			klog.Warningf("Failed to update cpu_cycles_event_reader map: %v", err)
			return nil, err
		}
	}
	for i, fd := range events.cpuInstructionsPerfEvents {
		if err = cpuInstructionsMap.Update(uint32(i), uint32(fd), ebpf.UpdateAny); err != nil {
			klog.Warningf("Failed to update cpu_instructions_event_reader map: %v", err)
			return nil, err
		}
	}
	for i, fd := range events.cacheMissPerfEvents {
		if err = cacheMissMap.Update(uint32(i), uint32(fd), ebpf.UpdateAny); err != nil {
			klog.Warningf("Failed to update cache_miss_event_reader map: %v", err)
			return nil, err
		}
	}
	return events, nil
}
