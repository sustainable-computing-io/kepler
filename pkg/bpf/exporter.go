//go:build linux && libbpf
// +build linux,libbpf

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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
	"unsafe"

	bpf "github.com/aquasecurity/libbpfgo"
	"github.com/jaypipes/ghw"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

const (
	objectFilename     = "kepler.%s.o"
	bpfAssestsLocation = "/var/lib/kepler/bpfassets"
	cpuOnline          = "/sys/devices/system/cpu/online"
	bpfPerfArraySuffix = "_event_reader"
	TableProcessName   = "processes"
	TableCPUFreqName   = "cpu_freq_array"
	MapSize            = 10240
	CPUNumSize         = 128
)

type exporter struct {
	module                *bpf.Module
	counters              map[string]perfCounter
	ebpfBatchGet          bool
	ebpfBatchGetAndDelete bool
	cpuCores              int
	// due to performance reason we keep an empty struct to verify if a new read is also empty
	emptyct                 ProcessBPFMetrics
	byteOrder               binary.ByteOrder
	perfEventFds            []int
	enabledHardwareCounters sets.Set[string]
	enabledSoftwareCounters sets.Set[string]
}

func NewExporter() (Exporter, error) {
	e := &exporter{
		module:                  nil,
		ebpfBatchGet:            true,
		ebpfBatchGetAndDelete:   true,
		cpuCores:                getCPUCores(),
		emptyct:                 ProcessBPFMetrics{},
		byteOrder:               utils.DetermineHostByteOrder(),
		perfEventFds:            []int{},
		enabledHardwareCounters: sets.New[string](),
		enabledSoftwareCounters: sets.New[string](),
	}
	err := e.attach()
	if err != nil {
		e.Detach()
	}
	return e, err
}

type perfCounter struct {
	EvType   int
	EvConfig int
}

func (e *exporter) SupportedMetrics() SupportedMetrics {
	return SupportedMetrics{
		HardwareCounters: e.enabledHardwareCounters.Clone(),
		SoftwareCounters: e.enabledSoftwareCounters.Clone(),
	}
}

func getLibbpfObjectFilePath(byteOrder binary.ByteOrder) (string, error) {
	var endianness string
	if byteOrder == binary.LittleEndian {
		endianness = "bpfel"
	} else if byteOrder == binary.BigEndian {
		endianness = "bpfeb"
	}
	filename := fmt.Sprintf(objectFilename, endianness)
	bpfassetsPath := fmt.Sprintf("%s/%s", bpfAssestsLocation, filename)
	_, err := os.Stat(bpfassetsPath)
	if err != nil {
		// attempt to find the bpf assets in the same directory as the binary
		// this is useful for running locally
		var matches []string
		err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
			if info.Name() == filename {
				matches = append(matches, path)
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("failed to find bpf object file: %w", err)
		}
		if len(matches) < 1 {
			return "", fmt.Errorf("failed to find bpf object file: no matches found")
		}
		klog.Infof("found bpf object file: %s", matches[0])
		return matches[0], nil
	}
	return bpfassetsPath, nil
}

func (e *exporter) attach() error {
	libbpfObjectFilePath, err := getLibbpfObjectFilePath(e.byteOrder)
	if err != nil {
		return fmt.Errorf("failed to load module: %w", err)
	}

	e.module, err = bpf.NewModuleFromFile(libbpfObjectFilePath)
	if err != nil {
		return fmt.Errorf("failed to load eBPF module from libbpf object: %w", err)
	}

	// resize array entries
	klog.Infof("%d CPU cores detected. Resizing eBPF Perf Event Arrays", e.cpuCores)
	toResize := []string{
		"cpu_cycles_event_reader", "cpu_instructions_event_reader", "cache_miss_event_reader", "task_clock_ms_event_reader",
		"cpu_cycles", "cpu_instructions", "cache_miss", "cpu_freq_array", "task_clock",
	}
	for _, arrayName := range toResize {
		if err = resizeArrayEntries(e.module, arrayName, e.cpuCores); err != nil {
			return fmt.Errorf("failed to resize array %s: %w", arrayName, err)
		}
	}
	// set the sample rate, this must be done before loading the object
	sampleRate := config.BPFSampleRate

	if err := e.module.InitGlobalVariable("SAMPLE_RATE", int32(sampleRate)); err != nil {
		return fmt.Errorf("failed to set sample rate: %w", err)
	}

	if err := e.module.BPFLoadObject(); err != nil {
		return fmt.Errorf("failed to load eBPF object: %w", err)
	}

	// attach to kprobe__finish_task_switch kprobe function
	prog, err := e.module.GetProgram("kepler_sched_switch_trace")
	if err != nil {
		return fmt.Errorf("failed to get kepler_sched_switch_trace: %w", err)
	}

	if _, err = prog.AttachGeneric(); err != nil {
		klog.Infof("failed to attach tracepoint/sched/sched_switch: %v", err)
	} else {
		e.enabledSoftwareCounters[config.CPUTime] = struct{}{}
	}

	if config.ExposeIRQCounterMetrics {
		err := func() error {
			// attach softirq_entry tracepoint to kepler_irq_trace function
			irq_prog, err := e.module.GetProgram("kepler_irq_trace")
			if err != nil {
				return fmt.Errorf("could not get kepler_irq_trace: %w", err)
			}
			if _, err := irq_prog.AttachGeneric(); err != nil {
				return fmt.Errorf("could not attach irq/softirq_entry: %w", err)
			}
			e.enabledSoftwareCounters[config.IRQNetTXLabel] = struct{}{}
			e.enabledSoftwareCounters[config.IRQNetRXLabel] = struct{}{}
			e.enabledSoftwareCounters[config.IRQBlockLabel] = struct{}{}
			return nil
		}()
		if err != nil {
			klog.Warningf("IRQ tracing disabled: %v", err)
		}
	}

	// attach function
	page_write, err := e.module.GetProgram("kepler_write_page_trace")
	if err != nil {
		return fmt.Errorf("failed to get kepler_write_page_trace: %w", err)
	} else {
		_, err = page_write.AttachTracepoint("writeback", "writeback_dirty_folio")
		if err != nil {
			klog.Warningf("failed to attach tp/writeback/writeback_dirty_folio: %v. Kepler will not collect page cache write events. This will affect the DRAM power model estimation on VMs.", err)
		} else {
			e.enabledSoftwareCounters[config.PageCacheHit] = struct{}{}
		}
	}

	// attach function
	page_read, err := e.module.GetProgram("kepler_read_page_trace")
	if err != nil {
		return fmt.Errorf("failed to get kepler_read_page_trace: %v", err)
	} else {
		if _, err = page_read.AttachGeneric(); err != nil {
			klog.Warningf("failed to attach fentry/mark_page_accessed: %v. Kepler will not collect page cache read events. This will affect the DRAM power model estimation on VMs.", err)
		} else {
			e.enabledSoftwareCounters[config.PageCacheHit] = struct{}{}
		}
	}

	if !config.ExposeHardwareCounterMetrics {
		klog.Infof("Hardware counter metrics are disabled")
		return nil
	}

	// attach performance counter fd to BPF_PERF_EVENT_ARRAY
	hardwareCounters := map[string]perfCounter{
		config.CPUCycle: {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CPU_CYCLES},
		// CPURefCycles aren't populated from the eBPF programs
		// If this is a bug, we should fix that and bring this map back
		// config.CPURefCycle:    {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_REF_CPU_CYCLES, true},
		config.CPUInstruction: {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_INSTRUCTIONS},
		config.CacheMiss:      {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CACHE_MISSES},
	}

	cleanup := func() error {
		unixClosePerfEvents(e.perfEventFds)
		e.perfEventFds = []int{}
		e.enabledHardwareCounters.Clear()
		return nil
	}

	for arrayName, counter := range hardwareCounters {
		bpfPerfArrayName := arrayName + bpfPerfArraySuffix
		bpfMap, perfErr := e.module.GetMap(bpfPerfArrayName)
		if perfErr != nil {
			klog.Warningf("could not get ebpf map for perf event %s: %v\n", bpfPerfArrayName, perfErr)
			return cleanup()
		}
		fds, perfErr := unixOpenPerfEvent(counter.EvType, counter.EvConfig, e.cpuCores)
		if perfErr != nil {
			klog.Warningf("could not attach perf event %s: %v. Are you using a VM?\n", bpfPerfArrayName, perfErr)
			return cleanup()
		}
		for i, fd := range fds {
			err = bpfMap.Update(unsafe.Pointer(&i), unsafe.Pointer(&fd))
			if err != nil {
				klog.Warningf("failed to update bpf map: %v", err)
				return cleanup()
			}
		}
		e.perfEventFds = append(e.perfEventFds, fds...)
		e.enabledHardwareCounters[arrayName] = struct{}{}
	}

	// attach task clock perf event. this is a software counter, not a hardware counter
	bpfPerfArrayName := config.TaskClock + bpfPerfArraySuffix
	bpfMap, err := e.module.GetMap(bpfPerfArrayName)
	if err != nil {
		return fmt.Errorf("could not get ebpf map for perf event %s: %w", bpfPerfArrayName, err)
	}
	fds, perfErr := unixOpenPerfEvent(unix.PERF_TYPE_SOFTWARE, unix.PERF_COUNT_SW_TASK_CLOCK, e.cpuCores)
	if perfErr != nil {
		return fmt.Errorf("could not attach perf event %s: %w", bpfPerfArrayName, perfErr)
	}
	for i, fd := range fds {
		err = bpfMap.Update(unsafe.Pointer(&i), unsafe.Pointer(&fd))
		if err != nil {
			klog.Warningf("failed to update bpf map: %v", err)
			return cleanup()
		}
	}
	e.perfEventFds = append(e.perfEventFds, fds...)
	e.enabledSoftwareCounters[config.TaskClock] = struct{}{}

	klog.Infof("Successfully load eBPF module from libbpf object")
	return nil
}

func (e *exporter) Detach() {
	unixClosePerfEvents(e.perfEventFds)
	e.perfEventFds = []int{}
	if e.module != nil {
		e.module.Close()
		e.module = nil
	}
}

func (e *exporter) CollectProcesses() (processesData []ProcessBPFMetrics, err error) {
	processesData = []ProcessBPFMetrics{}
	if e.module == nil {
		// nil error should be threw at attachment point, return empty data
		return
	}
	var processes *bpf.BPFMap
	processes, err = e.module.GetMap(TableProcessName)
	if err != nil {
		return
	}
	if e.ebpfBatchGetAndDelete {
		processesData, err = e.libbpfCollectProcessBatchSingleHash(processes)
	} else {
		processesData, err = e.libbpfCollectProcessSingleHash(processes)
	}
	if err == nil {
		return
	} else {
		e.ebpfBatchGetAndDelete = false
		processesData, err = e.libbpfCollectProcessSingleHash(processes)
	}
	return
}

func (e *exporter) CollectCPUFreq() (cpuFreqData map[int32]uint64, err error) {
	cpuFreqData = make(map[int32]uint64)
	var cpuFreq *bpf.BPFMap
	cpuFreq, err = e.module.GetMap(TableCPUFreqName)
	if err != nil {
		return
	}
	//cpuFreqkeySize := int(unsafe.Sizeof(uint32Key))
	iterator := cpuFreq.Iterator()
	var freq uint32
	// keySize := int(unsafe.Sizeof(freq))
	retry := 0
	next := iterator.Next()
	for next {
		keyBytes := iterator.Key()
		cpu := int32(e.byteOrder.Uint32(keyBytes))
		data, getErr := cpuFreq.GetValue(unsafe.Pointer(&cpu))
		if getErr != nil {
			retry += 1
			if retry > config.MaxLookupRetry {
				klog.V(5).Infof("failed to get data: %v with max retry: %d \n", getErr, config.MaxLookupRetry)
				next = iterator.Next()
				retry = 0
			}
			continue
		}
		getErr = binary.Read(bytes.NewBuffer(data), e.byteOrder, &freq)
		if getErr != nil {
			klog.V(5).Infof("failed to decode received data: %v\n", getErr)
			next = iterator.Next()
			retry = 0
			continue
		}
		if retry > 0 {
			klog.V(5).Infof("successfully get data with retry=%d \n", retry)
		}
		cpuFreqData[cpu] = uint64(freq)
		next = iterator.Next()
		retry = 0
	}
	return
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
		fd, err := unix.PerfEventOpen(sysAttr, -1, int(i), -1, cloexecFlags)
		if fd < 0 {
			return nil, fmt.Errorf("failed to open bpf perf event on cpu %d: %w", i, err)
		}
		fds = append(fds, int(fd))
	}
	return fds, nil
}

func unixClosePerfEvents(fds []int) {
	for _, fd := range fds {
		unix.SetNonblock(fd, true)
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

func resizeArrayEntries(module *bpf.Module, name string, size int) error {
	m, err := module.GetMap(name)
	if err != nil {
		return err
	}

	if err = m.Resize(uint32(size)); err != nil {
		return err
	}

	if current := m.GetMaxEntries(); current != uint32(size) {
		return fmt.Errorf("failed to resize map %s, expected %d, returned %d", name, size, current)
	}

	return nil
}

// for an unkown reason, the GetValueAndDeleteBatch never return the error (os.IsNotExist) that indicates the end of the table
// but it is not a big problem since we request all possible keys that the map can store in a single request
func (e *exporter) libbpfCollectProcessBatchSingleHash(processes *bpf.BPFMap) ([]ProcessBPFMetrics, error) {
	start := time.Now()
	processesData := []ProcessBPFMetrics{}
	var err error
	keySize := 4 // the map key is uint32,  has 4 bytes
	entries := MapSize / keySize

	keys := make([]uint32, entries)
	nextKey := uint32(0)

	val, err := processes.GetValueAndDeleteBatch(unsafe.Pointer(&keys[0]), nil, unsafe.Pointer(&nextKey), uint32(entries))
	if err != nil {
		// os.IsNotExist means we reached the end of the table
		if !os.IsNotExist(err) {
			klog.V(5).Infof("GetValueAndDeleteBatch failed: %v. A partial value might have been collected.", err)
		}
	}
	for _, value := range val {
		buff := bytes.NewBuffer(value)
		if buff == nil {
			klog.V(4).Infof("failed to get data: buffer EOF\n")
			continue
		}
		var ct ProcessBPFMetrics
		getErr := binary.Read(buff, e.byteOrder, &ct)
		if getErr != nil {
			klog.V(1).Infof("failed to decode received data: %v\n", getErr)
			continue
		}
		if ct != e.emptyct {
			processesData = append(processesData, ct)
		}
	}
	klog.V(5).Infof("successfully get data with batch get and delete with %d pids in %v", len(processesData), time.Since(start))
	return processesData, err
}

func (e *exporter) libbpfCollectProcessSingleHash(processes *bpf.BPFMap) (processesData []ProcessBPFMetrics, err error) {
	iterator := processes.Iterator()
	var ct ProcessBPFMetrics
	keys := []uint32{}
	retry := 0
	next := iterator.Next()
	for next {
		keyBytes := iterator.Key()
		key := e.byteOrder.Uint32(keyBytes)
		data, getErr := processes.GetValue(unsafe.Pointer(&key))
		if getErr != nil {
			retry += 1
			if retry > config.MaxLookupRetry {
				klog.V(5).Infof("failed to get data: %v with max retry: %d \n", getErr, config.MaxLookupRetry)
				next = iterator.Next()
				retry = 0
			}
			continue
		}
		getErr = binary.Read(bytes.NewBuffer(data), e.byteOrder, &ct)
		if getErr != nil {
			klog.V(5).Infof("failed to decode received data: %v\n", getErr)
			next = iterator.Next()
			retry = 0
			continue
		}
		if retry > 0 {
			klog.V(5).Infof("successfully get data with retry=%d \n", retry)
		}
		processesData = append(processesData, ct)
		keys = append(keys, key)
		next = iterator.Next()
		retry = 0
	}
	for _, key := range keys {
		// TODO delete keys in batch
		processes.DeleteKey(unsafe.Pointer(&key))
	}
	return
}
