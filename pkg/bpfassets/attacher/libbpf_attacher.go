//go:build libbpf
// +build libbpf

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
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

const (
	objectFilename       = "kepler.bpf.o"
	bpfAssesstsLocation  = "/var/lib/kepler/bpfassets"
	bpfAssesstsLocalPath = "../../../bpfassets/libbpf/bpf.o"
	cpuOnline            = "/sys/devices/system/cpu/online"
	LibbpfBuilt          = true
)

var (
	libbpfModule   *bpf.Module = nil
	libbpfCounters             = map[string]perfCounter{
		CPUCycleLabel:       {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CPU_CYCLES, true},
		CPURefCycleLabel:    {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_REF_CPU_CYCLES, true},
		CPUInstructionLabel: {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_INSTRUCTIONS, true},
		CacheMissLabel:      {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CACHE_MISSES, true},
		TaskClockLabel:      {unix.PERF_TYPE_SOFTWARE, unix.PERF_COUNT_SW_TASK_CLOCK, true},
	}
	uint32Key uint32
	uint64Key uint64
	maxRetry  = config.MaxLookupRetry
	bpfArrays = []string{
		"cpu_cycles_event_reader", "cpu_ref_cycles_event_reader", "cpu_instructions_event_reader", "cache_miss_event_reader", "task_clock_ms_event_reader",
		"cpu_cycles", "cpu_ref_cycles", "cpu_instructions", "cache_miss", "cpu_freq_array", "task_clock",
	}
	cpuCores = getCPUCores()
	emptyct  = ProcessBPFMetrics{} // due to performance reason we keep an empty struct to verify if a new read is also empty
	ctsize   = int(unsafe.Sizeof(emptyct))
)

func getLibbpfObjectFilePath(arch string) (string, error) {
	bpfassetsPath := fmt.Sprintf("%s/%s_%s", bpfAssesstsLocation, arch, objectFilename)
	_, err := os.Stat(bpfassetsPath)
	if err != nil {
		var absPath string
		// try relative path
		absPath, err = filepath.Abs(bpfAssesstsLocalPath)
		if err != nil {
			return "", err
		}
		bpfassetsPath = fmt.Sprintf("%s/%s_%s", absPath, arch, objectFilename)
		_, err = os.Stat(bpfassetsPath)
		if err != nil {
			return "", err
		}
	}
	return bpfassetsPath, nil
}

func attachLibbpfModule() (*bpf.Module, error) {
	var err error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to attach the bpf program: %v", err)
			klog.Infoln(err)
		}
	}()
	var libbpfObjectFilePath string
	arch := runtime.GOARCH
	libbpfObjectFilePath, err = getLibbpfObjectFilePath(arch)
	if err == nil {
		libbpfModule, err = bpf.NewModuleFromFile(libbpfObjectFilePath)
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

	err = libbpfModule.InitGlobalVariable("sample_rate", int32(sampleRate))
	if err != nil {
		return nil, fmt.Errorf("failed to set sample rate: %v", err)
	}
	err = libbpfModule.BPFLoadObject()

	// attach to kprobe__finish_task_switch kprobe function
	prog, err := libbpfModule.GetProgram("kprobe__finish_task_switch")
	if err != nil {
		return libbpfModule, fmt.Errorf("failed to get kprobe__finish_task_switch: %v", err)
	} else {
		_, err = prog.AttachKprobe("finish_task_switch")
		if err != nil {
			// try finish_task_switch.isra.0
			klog.Infof("failed to attach kprobe/finish_task_switch: %v. Try finish_task_switch.isra.0", err)
			_, err = prog.AttachKprobe("finish_task_switch.isra.0")
			if err != nil {
				return libbpfModule, fmt.Errorf("failed to attach kprobe/finish_task_switch or finish_task_switch.isra.0: %v", err)
			}
		}
	}

	// attach softirq_entry tracepoint to kepler_irq_trace function
	irq_prog, err := libbpfModule.GetProgram("kepler_irq_trace")
	if err != nil {
		klog.Warningf("could not get kepler_irq_trace: %v", err)
		// disable IRQ metric
		config.ExposeIRQCounterMetrics = false
	} else {
		_, err = irq_prog.AttachTracepoint("irq", "softirq_entry")
		if err != nil {
			klog.Warningf("could not attach irq/softirq_entry: %v", err)
			// disable IRQ metric
			config.ExposeIRQCounterMetrics = false
		}
	}

	// attach function
	page_write, err := libbpfModule.GetProgram("kprobe__set_page_dirty")
	if err != nil {
		return libbpfModule, fmt.Errorf("failed to get kprobe__set_page_dirty: %v", err)
	} else {
		_, err = page_write.AttachKprobe("set_page_dirty")
		if err != nil {
			_, err = page_write.AttachKprobe("mark_buffer_dirty")
			if err != nil {
				klog.Warningf("failed to attach kprobe/set_page_dirty or mark_buffer_dirty: %v. Kepler will not collect page cache write events. This will affect the DRAM power model estimation on VMs.", err)
			}
		}
	}

	// attach function
	page_read, err := libbpfModule.GetProgram("kprobe__mark_page_accessed")
	if err != nil {
		return libbpfModule, fmt.Errorf("failed to get kprobe__mark_page_accessed: %v", err)
	} else {
		_, err = page_read.AttachKprobe("mark_page_accessed")
		if err != nil {
			klog.Warningf("failed to attach kprobe/mark_page_accessed: %v. Kepler will not collect page cache read events. This will affect the DRAM power model estimation on VMs.", err)
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

func detachLibbpfModule() {
	unixClosePerfEvent()
	if libbpfModule != nil {
		libbpfModule.Close()
		libbpfModule = nil
	}
}

func libbpfCollectProcess() (processesData []ProcessBPFMetrics, err error) {
	processesData = []ProcessBPFMetrics{}
	if libbpfModule == nil {
		// nil error should be threw at attachment point, return empty data
		return
	}
	var processes *bpf.BPFMap
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

func libbpfCollectFreq() (cpuFreqData map[int32]uint64, err error) {
	cpuFreqData = make(map[int32]uint64)
	var cpuFreq *bpf.BPFMap
	cpuFreq, err = libbpfModule.GetMap(TableCPUFreqName)
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
	return
}

///////////////////////////////////////////////////////////////////////////
// utility functions

func unixOpenPerfEvent(bpfMap *bpf.BPFMap, typ, conf int) error {
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

		fd, err := unix.PerfEventOpen(sysAttr, -1, int(i), -1, cloexecFlags)
		if fd < 0 {
			return fmt.Errorf("failed to open bpf perf event on cpu %d: %v", i, err)
		}
		err = bpfMap.Update(unsafe.Pointer(&i), unsafe.Pointer(&fd))
		if err != nil {
			return fmt.Errorf("failed to update bpf map: %v", err)
		}
		res = append(res, int(fd))
	}

	PerfEvents[perfKey] = res

	return nil
}

func unixClosePerfEvent() {
	for _, vs := range PerfEvents {
		for _, fd := range vs {
			unix.SetNonblock(fd, true)
			unix.Close(fd)
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
func libbpfCollectProcessBatchSingleHash(processes *bpf.BPFMap) ([]ProcessBPFMetrics, error) {
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

func libbpfCollectProcessSingleHash(processes *bpf.BPFMap) (processesData []ProcessBPFMetrics, err error) {
	iterator := processes.Iterator()
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
				next = iterator.Next()
				retry = 0
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
		processes.DeleteKey(unsafe.Pointer(&key))
	}
	return
}
