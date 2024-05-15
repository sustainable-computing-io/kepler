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

package attacher

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
	"golang.org/x/exp/slices"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

const (
	objectFilename     = "kepler.%s.o"
	bpfAssestsLocation = "/var/lib/kepler/bpfassets"
	cpuOnline          = "/sys/devices/system/cpu/online"
	bpfPerfArraySuffix = "_event_reader"

	// Per /sys/kernel/debug/tracing/events/irq/softirq_entry/format
	// { 0, "HI" }, { 1, "TIMER" }, { 2, "NET_TX" }, { 3, "NET_RX" }, { 4, "BLOCK" }, { 5, "IRQ_POLL" }, { 6, "TASKLET" }, { 7, "SCHED" }, { 8, "HRTIMER" }, { 9, "RCU" }

	// IRQ vector to IRQ number
	IRQNetTX = 2
	IRQNetRX = 3
	IRQBlock = 4

	TableProcessName = "processes"
	TableCPUFreqName = "cpu_freq_array"
	MapSize          = 10240
	CPUNumSize       = 128
)

type attacher struct {
	module                *bpf.Module
	counters              map[string]perfCounter
	ebpfBatchGet          bool
	ebpfBatchGetAndDelete bool
	cpuCores              int
	// due to performance reason we keep an empty struct to verify if a new read is also empty
	emptyct                 ProcessBPFMetrics
	hardwareCountersEnabled bool
	byteOrder               binary.ByteOrder
	perfEventFds            []int
	enabledHardwareCounters []string
	enabledSoftwareCounters []string
}

func NewAttacher() (Attacher, error) {
	a := &attacher{
		module:                  nil,
		ebpfBatchGet:            true,
		ebpfBatchGetAndDelete:   true,
		cpuCores:                getCPUCores(),
		emptyct:                 ProcessBPFMetrics{},
		hardwareCountersEnabled: true,
		byteOrder:               utils.DetermineHostByteOrder(),
		perfEventFds:            []int{},
		enabledHardwareCounters: []string{},
		enabledSoftwareCounters: []string{},
	}
	err := a.attach()
	if err != nil {
		a.Detach()
	}
	return a, err
}

type perfCounter struct {
	EvType   int
	EvConfig int
}

func (a *attacher) GetEnabledBPFHWCounters() []string {
	return a.enabledHardwareCounters
}

func (a *attacher) GetEnabledBPFSWCounters() []string {
	return a.enabledSoftwareCounters
}

func (a *attacher) HardwareCountersEnabled() bool {
	return a.hardwareCountersEnabled
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
			return "", fmt.Errorf("failed to find bpf object file: %v", err)
		}
		if len(matches) < 1 {
			return "", fmt.Errorf("failed to find bpf object file: no matches found")
		}
		klog.Infof("found bpf object file: %s", matches[0])
		return matches[0], nil
	}
	return bpfassetsPath, nil
}

func (a *attacher) attach() error {
	libbpfObjectFilePath, err := getLibbpfObjectFilePath(a.byteOrder)
	if err != nil {
		return fmt.Errorf("failed to load module: %v", err)
	}

	a.module, err = bpf.NewModuleFromFile(libbpfObjectFilePath)
	if err != nil {
		return fmt.Errorf("failed to load eBPF module from libbpf object: %v", err)
	}

	// resize array entries
	klog.Infof("%d CPU cores detected. Resizing eBPF Perf Event Arrays", a.cpuCores)
	toResize := []string{
		"cpu_cycles_event_reader", "cpu_instructions_event_reader", "cache_miss_event_reader", "task_clock_ms_event_reader",
		"cpu_cycles", "cpu_instructions", "cache_miss", "cpu_freq_array", "task_clock",
	}
	for _, arrayName := range toResize {
		if err = resizeArrayEntries(a.module, arrayName, a.cpuCores); err != nil {
			return fmt.Errorf("failed to resize array %s: %v\n", arrayName, err)
		}
	}
	// set the sample rate, this must be done before loading the object
	sampleRate := config.BPFSampleRate

	if err := a.module.InitGlobalVariable("SAMPLE_RATE", int32(sampleRate)); err != nil {
		return fmt.Errorf("failed to set sample rate: %v", err)
	}

	if err := a.module.BPFLoadObject(); err != nil {
		return fmt.Errorf("failed to load eBPF object: %v", err)
	}

	// attach to kprobe__finish_task_switch kprobe function
	prog, err := a.module.GetProgram("kepler_sched_switch_trace")
	if err != nil {
		return fmt.Errorf("failed to get kepler_sched_switch_trace: %v", err)
	}

	if _, err = prog.AttachGeneric(); err != nil {
		klog.Infof("failed to attach tracepoint/sched/sched_switch: %v", err)
	} else {
		a.enabledSoftwareCounters = append(a.enabledSoftwareCounters, config.CPUTime)
	}

	if config.ExposeIRQCounterMetrics {
		// attach softirq_entry tracepoint to kepler_irq_trace function
		irq_prog, err := a.module.GetProgram("kepler_irq_trace")
		if err != nil {
			klog.Warningf("could not get kepler_irq_trace: %v", err)
			// disable IRQ metric
			config.ExposeIRQCounterMetrics = false
		} else {
			if _, err := irq_prog.AttachGeneric(); err != nil {
				klog.Warningf("could not attach irq/softirq_entry: %v", err)
				// disable IRQ metric
				config.ExposeIRQCounterMetrics = false
			}
			a.enabledSoftwareCounters = append(a.enabledSoftwareCounters, SoftIRQEvents...)
		}
	}

	// attach function
	page_write, err := a.module.GetProgram("kepler_write_page_trace")
	if err != nil {
		return fmt.Errorf("failed to get kepler_write_page_trace: %v", err)
	} else {
		_, err = page_write.AttachTracepoint("writeback", "writeback_dirty_folio")
		if err != nil {
			klog.Warningf("failed to attach tp/writeback/writeback_dirty_folio: %v. Kepler will not collect page cache write events. This will affect the DRAM power model estimation on VMs.", err)
		} else {
			a.enabledSoftwareCounters = append(a.enabledSoftwareCounters, config.PageCacheHit)
		}
	}

	// attach function
	page_read, err := a.module.GetProgram("kepler_read_page_trace")
	if err != nil {
		return fmt.Errorf("failed to get kepler_read_page_trace: %v", err)
	} else {
		if _, err = page_read.AttachGeneric(); err != nil {
			klog.Warningf("failed to attach fentry/mark_page_accessed: %v. Kepler will not collect page cache read events. This will affect the DRAM power model estimation on VMs.", err)
		} else {
			if !slices.Contains(a.enabledSoftwareCounters, config.PageCacheHit) {
				a.enabledSoftwareCounters = append(a.enabledSoftwareCounters, config.PageCacheHit)
			}
		}
	}

	// attach performance counter fd to BPF_PERF_EVENT_ARRAY
	counters := map[string]perfCounter{
		config.CPUCycle: {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CPU_CYCLES},
		// CPURefCycles aren't populated from the eBPF programs
		// If this is a bug, we should fix that and bring this map back
		// config.CPURefCycle:    {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_REF_CPU_CYCLES, true},
		config.CPUInstruction: {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_INSTRUCTIONS},
		config.CacheMiss:      {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CACHE_MISSES},
		config.TaskClock:      {unix.PERF_TYPE_SOFTWARE, unix.PERF_COUNT_SW_TASK_CLOCK},
	}

	for arrayName, counter := range counters {
		bpfPerfArrayName := arrayName + bpfPerfArraySuffix
		bpfMap, perfErr := a.module.GetMap(bpfPerfArrayName)
		if perfErr != nil {
			klog.Warningf("could not get ebpf map for perf event %s: %v\n", bpfPerfArrayName, perfErr)
			continue
		} else {
			fds, perfErr := unixOpenPerfEvent(counter.EvType, counter.EvConfig, a.cpuCores)
			if perfErr != nil {
				// some hypervisors don't expose perf counters
				klog.Warningf("could not attach perf event %s: %v. Are you using a VM?\n", bpfPerfArrayName, perfErr)
				// if any counter is not enabled, we need disable HardwareCountersEnabled
				a.hardwareCountersEnabled = false
			}
			for i, fd := range fds {
				err = bpfMap.Update(unsafe.Pointer(&i), unsafe.Pointer(&fd))
				if err != nil {
					return fmt.Errorf("failed to update bpf map: %v", err)
				}
			}
			a.perfEventFds = append(a.perfEventFds, fds...)
			a.enabledHardwareCounters = append(a.enabledHardwareCounters, arrayName)
		}
	}
	klog.Infof("Successfully load eBPF module from libbpf object")
	return nil
}

func (a *attacher) Detach() {
	unixClosePerfEvents(a.perfEventFds)
	a.perfEventFds = []int{}
	if a.module != nil {
		a.module.Close()
		a.module = nil
	}
}

func (a *attacher) CollectProcesses() (processesData []ProcessBPFMetrics, err error) {
	processesData = []ProcessBPFMetrics{}
	if a.module == nil {
		// nil error should be threw at attachment point, return empty data
		return
	}
	var processes *bpf.BPFMap
	processes, err = a.module.GetMap(TableProcessName)
	if err != nil {
		return
	}
	if a.ebpfBatchGetAndDelete {
		processesData, err = a.libbpfCollectProcessBatchSingleHash(processes)
	} else {
		processesData, err = a.libbpfCollectProcessSingleHash(processes)
	}
	if err == nil {
		return
	} else {
		a.ebpfBatchGetAndDelete = false
		processesData, err = a.libbpfCollectProcessSingleHash(processes)
	}
	return
}

func (a *attacher) CollectCPUFreq() (cpuFreqData map[int32]uint64, err error) {
	cpuFreqData = make(map[int32]uint64)
	var cpuFreq *bpf.BPFMap
	cpuFreq, err = a.module.GetMap(TableCPUFreqName)
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
		cpu := int32(a.byteOrder.Uint32(keyBytes))
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
		getErr = binary.Read(bytes.NewBuffer(data), a.byteOrder, &freq)
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
			return nil, fmt.Errorf("failed to open bpf perf event on cpu %d: %v", i, err)
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
func (a *attacher) libbpfCollectProcessBatchSingleHash(processes *bpf.BPFMap) ([]ProcessBPFMetrics, error) {
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
		getErr := binary.Read(buff, a.byteOrder, &ct)
		if getErr != nil {
			klog.V(1).Infof("failed to decode received data: %v\n", getErr)
			continue
		}
		if ct != a.emptyct {
			processesData = append(processesData, ct)
		}
	}
	klog.V(5).Infof("successfully get data with batch get and delete with %d pids in %v", len(processesData), time.Since(start))
	return processesData, err
}

func (a *attacher) libbpfCollectProcessSingleHash(processes *bpf.BPFMap) (processesData []ProcessBPFMetrics, err error) {
	iterator := processes.Iterator()
	var ct ProcessBPFMetrics
	keys := []uint32{}
	retry := 0
	next := iterator.Next()
	for next {
		keyBytes := iterator.Key()
		key := a.byteOrder.Uint32(keyBytes)
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
		getErr = binary.Read(bytes.NewBuffer(data), a.byteOrder, &ct)
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
