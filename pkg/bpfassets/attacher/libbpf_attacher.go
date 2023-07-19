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
	bpf "github.com/aquasecurity/tracee/libbpfgo"
	"github.com/jaypipes/ghw"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"
)

const (
	objectFilename       = "kepler.bpf.o"
	bpfAssesstsLocation  = "/var/lib/kepler/bpfassets"
	bpfAssesstsLocalPath = "../../../bpfassets/libbpf/bpf.o"
	cpuOnline            = "/sys/devices/system/cpu/online"

	LibbpfBuilt = true
	maxRetry    = 500
)

var (
	libbpfModule   *bpf.Module = nil
	libbpfCounters             = map[string]perfCounter{
		CPUCycleLabel:       {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CPU_CYCLES, true},
		CPURefCycleLabel:    {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_REF_CPU_CYCLES, true},
		CPUInstructionLabel: {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_INSTRUCTIONS, true},
		CacheMissLabel:      {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CACHE_MISSES, true},
	}
	uint32Key uint32
	uint64Key uint64
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

	err = libbpfModule.BPFLoadObject()

	// attach sched_switch tracepoint to kepler_trace function
	prog, err := libbpfModule.GetProgram("kepler_trace")
	if err != nil {
		return libbpfModule, fmt.Errorf("failed to get kepler_trace: %v", err)
	} else {
		_, err = prog.AttachTracepoint("sched:sched_switch")
		if err != nil {
			return libbpfModule, fmt.Errorf("failed to attach sched/sched_switch: %v", err)
		}
	}

	// attach softirq_entry tracepoint to kepler_irq_trace function
	irq_prog, err := libbpfModule.GetProgram("kepler_irq_trace")
	if err != nil {
		klog.Infof("failed to get kepler_irq_trace: %v", err)
		// disable IRQ metric
		config.ExposeIRQCounterMetrics = false
	} else {
		_, err = irq_prog.AttachTracepoint("irq:softirq_entry")
		if err != nil {
			klog.Infof("failed to attach irq/softirq_entry: %v", err)
			// disable IRQ metric
			config.ExposeIRQCounterMetrics = false
		}
	}

	// attach performance counter fd to BPF_PERF_EVENT_ARRAY
	for arrayName, counter := range Counters {
		bpfPerfArrayName := arrayName + BpfPerfArrayPrefix
		bpfMap, perfErr := libbpfModule.GetMap(bpfPerfArrayName)
		if perfErr != nil {
			klog.Infof("failed to get perf event %s: %v\n", bpfPerfArrayName, perfErr)
			continue
		}
		perfErr = unixOpenPerfEvent(bpfMap, counter.EvType, counter.EvConfig)
		if perfErr != nil {
			// some hypervisors don't expose perf counters
			klog.Infof("failed to attach perf event %s: %v\n", bpfPerfArrayName, perfErr)
			counter.enabled = false

			// if any counter is not enabled, we need disable HardwareCountersEnabled
			HardwareCountersEnabled = false
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
	processKeySize := int(unsafe.Sizeof(uint64Key))
	iterator := processes.Iterator(processKeySize)
	var ct ProcessBPFMetrics
	valueSize := int(unsafe.Sizeof(ProcessBPFMetrics{}))
	keys := []uint32{}
	retry := 0
	next := iterator.Next()
	for next {
		keyBytes := iterator.Key()
		key := ByteOrder.Uint32(keyBytes)
		data, getErr := processes.GetValue(key, valueSize)
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
		processes.DeleteKey(key)
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
	cpuFreqkeySize := int(unsafe.Sizeof(uint32Key))
	iterator := cpuFreq.Iterator(cpuFreqkeySize)
	var freq uint32
	valueSize := int(unsafe.Sizeof(freq))
	retry := 0
	next := iterator.Next()
	for next {
		keyBytes := iterator.Key()
		cpu := int32(ByteOrder.Uint32(keyBytes))
		data, getErr := cpuFreq.GetValue(cpu, valueSize)
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

	cores := runtime.NumCPU()
	if cpu, err := ghw.CPU(); err == nil {
		// we need to get the number of all CPUs,
		// so if /proc/cpuinfo is available, we can get the number of all CPUs
		cores = int(cpu.TotalThreads)
	}

	res := []int{}

	for i := 0; i < cores; i++ {
		cloexecFlags := unix.PERF_FLAG_FD_CLOEXEC

		fd, err := unix.PerfEventOpen(sysAttr, -1, int(i), -1, cloexecFlags)
		if fd < 0 {
			return fmt.Errorf("failed to open bpf perf event on cpu %d: %v", i, err)
		}
		err = bpfMap.Update(int32(i), uint32(fd))
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
