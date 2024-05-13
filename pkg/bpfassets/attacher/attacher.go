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

	"github.com/aquasecurity/libbpfgo"
	"github.com/jaypipes/ghw"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/utils"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

const (
	objectFilename       = "kepler.%s.o"
	bpfAssesstsLocation  = "/var/lib/kepler/bpfassets"
	bpfAssesstsLocalPath = "../../../bpfassets/libbpf/bpf.o"
	cpuOnline            = "/sys/devices/system/cpu/online"
	CPUCycleLabel        = config.CPUCycle
	CPURefCycleLabel     = config.CPURefCycle
	CPUInstructionLabel  = config.CPUInstruction
	CacheMissLabel       = config.CacheMiss
	TaskClockLabel       = config.TaskClock

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

var (
	libbpfModule   *libbpfgo.Module = nil
	libbpfCounters                  = map[string]perfCounter{
		CPUCycleLabel: {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CPU_CYCLES, true},
		// CPURefCycles aren't populated from the eBPF programs
		// If this is a bug, we should fix that and bring this map back
		// CPURefCycleLabel:    {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_REF_CPU_CYCLES, true},
		CPUInstructionLabel: {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_INSTRUCTIONS, true},
		CacheMissLabel:      {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CACHE_MISSES, true},
		TaskClockLabel:      {unix.PERF_TYPE_SOFTWARE, unix.PERF_COUNT_SW_TASK_CLOCK, true},
	}
	maxRetry  = config.MaxLookupRetry
	bpfArrays = []string{
		"cpu_cycles_event_reader", "cpu_instructions_event_reader", "cache_miss_event_reader", "task_clock_ms_event_reader",
		"cpu_cycles", "cpu_instructions", "cache_miss", "cpu_freq_array", "task_clock",
	}
	cpuCores = getCPUCores()
	emptyct  = ProcessBPFMetrics{} // due to performance reason we keep an empty struct to verify if a new read is also empty

	// ebpfBatchGet is true if the kernel supports batch get operation
	ebpfBatchGet = true
	// ebpfBatchGetAndDelete is true if delete all the keys after batch get
	ebpfBatchGetAndDelete = ebpfBatchGet

	Counters                map[string]perfCounter
	HardwareCountersEnabled = true
	BpfPerfArrayPrefix      = "_event_reader"

	PerfEvents = map[string][]int{}
	ByteOrder  binary.ByteOrder

	SoftIRQEvents = []string{config.IRQNetTXLabel, config.IRQNetRXLabel, config.IRQBlockLabel}
)

type perfCounter struct {
	EvType   int
	EvConfig int
	enabled  bool
}

func init() {
	ByteOrder = utils.DetermineHostByteOrder()
}

func getCounters() map[string]perfCounter {
	return libbpfCounters
}

func GetEnabledBPFHWCounters() []string {
	Counters = getCounters()
	var metrics []string
	klog.V(5).Infof("hardeware counter metrics config %t", config.IsHCMetricsEnabled())
	if !config.IsHCMetricsEnabled() {
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

func GetEnabledBPFSWCounters() []string {
	var metrics []string
	metrics = append(metrics, config.CPUTime, config.PageCacheHit)

	klog.V(5).Infof("irq counter metrics config %t", config.ExposeIRQCounterMetrics)
	if !config.ExposeIRQCounterMetrics {
		klog.V(5).Info("irq counter metrics not enabled")
		return metrics
	}
	metrics = append(metrics, SoftIRQEvents...)
	return metrics
}

func getLibbpfObjectFilePath() (string, error) {
	var endianness string
	if ByteOrder == binary.LittleEndian {
		endianness = "bpfel"
	} else if ByteOrder == binary.BigEndian {
		endianness = "bpfeb"
	}
	filename := fmt.Sprintf(objectFilename, endianness)
	bpfassetsPath := fmt.Sprintf("%s/%s", bpfAssesstsLocation, filename)
	_, err := os.Stat(bpfassetsPath)
	if err != nil {
		var absPath string
		// try relative path
		absPath, err = filepath.Abs(bpfAssesstsLocalPath)
		if err != nil {
			return "", err
		}
		bpfassetsPath = fmt.Sprintf("%s/%s", absPath, filename)
		_, err = os.Stat(bpfassetsPath)
		if err != nil {
			return "", err
		}
	}
	return bpfassetsPath, nil
}

func Attach() (*libbpfgo.Module, error) {
	m, err := attachLibbpfModule()
	if err != nil {
		Detach()
		klog.Infof("failed to attach bpf with libbpf: %v", err)
		return nil, err
	}
	return m, nil
}

func attachLibbpfModule() (*libbpfgo.Module, error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to attach the bpf program: %v", err)
			klog.Infoln(err)
		}
	}()
	var libbpfObjectFilePath string
	libbpfObjectFilePath, err = getLibbpfObjectFilePath()
	if err == nil {
		libbpfModule, err = libbpfgo.NewModuleFromFile(libbpfObjectFilePath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load module: %v", err)
	}
	// resize array entries
	for _, arrayName := range bpfArrays {
		err = resizeArrayEntries(arrayName, cpuCores)
		if err != nil {
			klog.Infof("failed to resize array %s: %v\n", arrayName, err)
		}
	}
	// set the sample rate, this must be done before loading the object
	sampleRate := config.BPFSampleRate

	err = libbpfModule.InitGlobalVariable("SAMPLE_RATE", int32(sampleRate))
	if err != nil {
		return nil, fmt.Errorf("failed to set sample rate: %v", err)
	}
	err = libbpfModule.BPFLoadObject()

	// attach to kprobe__finish_task_switch kprobe function
	prog, err := libbpfModule.GetProgram("kepler_sched_switch_trace")
	if err != nil {
		return libbpfModule, fmt.Errorf("failed to get kepler_sched_switch_trace: %v", err)
	} else {
		_, err = prog.AttachGeneric()
		if err != nil {
			// try finish_task_switch.isra.0
			klog.Infof("failed to attach tracepoint/sched/sched_switch: %v", err)
		}
	}

	// attach softirq_entry tracepoint to kepler_irq_trace function
	irqProg, err := libbpfModule.GetProgram("kepler_irq_trace")
	if err != nil {
		klog.Warningf("could not get kepler_irq_trace: %v", err)
		// disable IRQ metric
		config.ExposeIRQCounterMetrics = false
	} else {
		_, err = irqProg.AttachTracepoint("irq", "softirq_entry")
		if err != nil {
			klog.Warningf("could not attach irq/softirq_entry: %v", err)
			// disable IRQ metric
			config.ExposeIRQCounterMetrics = false
		}
	}

	// attach function
	pageWrite, err := libbpfModule.GetProgram("kepler_write_page_trace")
	if err != nil {
		return libbpfModule, fmt.Errorf("failed to get kepler_write_page_trace: %v", err)
	} else {
		_, err = pageWrite.AttachTracepoint("writeback", "writeback_dirty_folio")
		if err != nil {
			klog.Warningf("failed to attach tp/writeback/writeback_dirty_folio: %v. Kepler will not collect page cache write events. This will affect the DRAM power model estimation on VMs.", err)
		}
	}

	// attach function
	pageRead, err := libbpfModule.GetProgram("kepler_read_page_trace")
	if err != nil {
		return libbpfModule, fmt.Errorf("failed to get kepler_read_page_trace: %v", err)
	} else {
		if _, err = pageRead.AttachGeneric(); err != nil {
			klog.Warningf("failed to attach fentry/mark_page_accessed: %v. Kepler will not collect page cache read events. This will affect the DRAM power model estimation on VMs.", err)
		}
	}

	// attach performance counter fd to BPF_PERF_EVENT_ARRAY
	for arrayName, counter := range Counters {
		bpfPerfArrayName := arrayName + BpfPerfArrayPrefix
		bpfMap, perfErr := libbpfModule.GetMap(bpfPerfArrayName)
		if perfErr != nil {
			klog.Warningf("could not get perf event %s: %v. Are you using a VM?\n", bpfPerfArrayName, perfErr)
			continue
		} else {
			perfErr = unixOpenPerfEvent(bpfMap, counter.EvType, counter.EvConfig)
			if perfErr != nil {
				// some hypervisors don't expose perf counters
				klog.Warningf("could not attach perf event %s: %v. Are you using a VM?\n", bpfPerfArrayName, perfErr)
				counter.enabled = false

				// if any counter is not enabled, we need disable HardwareCountersEnabled
				HardwareCountersEnabled = false
			}
		}
	}
	klog.Infof("Successfully load eBPF module from libbpf object")
	return libbpfModule, nil
}

func Detach() {
	unixClosePerfEvent()
	if libbpfModule != nil {
		libbpfModule.Close()
		libbpfModule = nil
	}
}

func CollectProcesses() (processesData []ProcessBPFMetrics, err error) {
	processesData = []ProcessBPFMetrics{}
	if libbpfModule == nil {
		// nil error should be threw at attachment point, return empty data
		return
	}
	var processes *libbpfgo.BPFMap
	processes, err = libbpfModule.GetMap(TableProcessName)
	if err != nil {
		return
	}
	if ebpfBatchGetAndDelete {
		processesData, err = libbpfCollectProcessBatchSingleHash(processes)
	} else {
		processesData, err = libbpfCollectProcessSingleHash(processes)
	}
	if err == nil {
		return
	} else {
		ebpfBatchGetAndDelete = false
		processesData, err = libbpfCollectProcessSingleHash(processes)
	}
	return
}

func CollectCPUFreq() (map[int32]uint64, error) {
	cpuFreqData := make(map[int32]uint64)
	cpuFreq, err := libbpfModule.GetMap(TableCPUFreqName)
	if err != nil {
		return nil, err
	}
	iterator := cpuFreq.Iterator()
	var freq uint32
	retry := 0
	next := iterator.Next()
	for next {
		keyBytes := iterator.Key()
		cpu := int32(ByteOrder.Uint32(keyBytes))
		data, getErr := cpuFreq.GetValue(unsafe.Pointer(&cpu))
		if getErr != nil {
			retry += 1
			if retry > maxRetry {
				klog.V(5).Infof("failed to get data: %v with max retry: %d \n", getErr, maxRetry)
				next = iterator.Next()
				retry = 0
			}
			continue
		}
		getErr = binary.Read(bytes.NewBuffer(data), ByteOrder, &freq)
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
	return cpuFreqData, nil
}

///////////////////////////////////////////////////////////////////////////
// utility functions

func unixOpenPerfEvent(bpfMap *libbpfgo.BPFMap, typ, conf int) error {
	perfKey := fmt.Sprintf("%d:%d", typ, conf)
	sysAttr := &unix.PerfEventAttr{
		Type:   uint32(typ),
		Size:   uint32(unsafe.Sizeof(unix.PerfEventAttr{})),
		Config: uint64(conf),
	}

	if _, ok := PerfEvents[perfKey]; ok {
		return nil
	}

	res := []int{}

	for i := 0; i < cpuCores; i++ {
		cloexecFlags := unix.PERF_FLAG_FD_CLOEXEC

		fd, err := unix.PerfEventOpen(sysAttr, -1, i, -1, cloexecFlags)
		if fd < 0 {
			return fmt.Errorf("failed to open bpf perf event on cpu %d: %v", i, err)
		}
		err = bpfMap.Update(unsafe.Pointer(&i), unsafe.Pointer(&fd))
		if err != nil {
			return fmt.Errorf("failed to update bpf map: %v", err)
		}
		res = append(res, fd)
	}

	PerfEvents[perfKey] = res

	return nil
}

func unixClosePerfEvent() {
	for _, vs := range PerfEvents {
		for _, fd := range vs {
			if err := unix.SetNonblock(fd, true); err != nil {
				klog.Warningf("failed to set nonblock: %v", err)
			}
			if err := unix.Close(fd); err != nil {
				klog.Warningf("failed to close perf event: %v", err)
			}
		}
	}
	PerfEvents = map[string][]int{}
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

func resizeArrayEntries(name string, size int) error {
	m, err := libbpfModule.GetMap(name)
	if err != nil {
		return err
	}

	if err := m.Resize(uint32(size)); err != nil {
		return err
	}

	if current := m.GetMaxEntries(); current != uint32(size) {
		return fmt.Errorf("failed to resize map %s, expected %d, returned %d", name, size, current)
	}

	return nil
}

// for an unknown reason, the GetValueAndDeleteBatch never return the error (os.IsNotExist) that indicates the end of the table
// but it is not a big problem since we request all possible keys that the map can store in a single request
func libbpfCollectProcessBatchSingleHash(processes *libbpfgo.BPFMap) ([]ProcessBPFMetrics, error) {
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
		getErr := binary.Read(buff, ByteOrder, &ct)
		if getErr != nil {
			klog.V(1).Infof("failed to decode received data: %v\n", getErr)
			continue
		}
		if ct != emptyct {
			processesData = append(processesData, ct)
		}
	}
	klog.V(5).Infof("successfully get data with batch get and delete with %d pids in %v", len(processesData), time.Since(start))
	return processesData, err
}

func libbpfCollectProcessSingleHash(processes *libbpfgo.BPFMap) ([]ProcessBPFMetrics, error) {
	iterator := processes.Iterator()
	processesData := []ProcessBPFMetrics{}
	var ct ProcessBPFMetrics
	keys := []uint32{}
	retry := 0
	next := iterator.Next()
	for next {
		keyBytes := iterator.Key()
		key := ByteOrder.Uint32(keyBytes)
		data, getErr := processes.GetValue(unsafe.Pointer(&key))
		if getErr != nil {
			retry += 1
			if retry > maxRetry {
				klog.V(5).Infof("failed to get data: %v with max retry: %d \n", getErr, maxRetry)
				return processesData, getErr
			}
			continue
		}
		getErr = binary.Read(bytes.NewBuffer(data), ByteOrder, &ct)
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
		if err := processes.DeleteKey(unsafe.Pointer(&key)); err != nil {
			klog.Errorf("failed to delete key %d: %v", key, err)
		}
	}
	return processesData, nil
}
