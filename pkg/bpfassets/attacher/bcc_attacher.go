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
	"os"
	"runtime"
	"strconv"

	"bytes"
	"encoding/binary"
	"unsafe"

	"github.com/iovisor/gobpf/pkg/cpuonline"
	assets "github.com/sustainable-computing-io/kepler/pkg/bpfassets"
	"github.com/sustainable-computing-io/kepler/pkg/config"

	bpf "github.com/iovisor/gobpf/bcc"
	"github.com/jaypipes/ghw"
	"golang.org/x/sys/unix"

	"k8s.io/klog/v2"
)

/*
#cgo CFLAGS: -I/usr/include/bcc/compat
#cgo LDFLAGS: -lbcc
#include <bcc/bcc_common.h>
#include <bcc/libbpf.h>
#include <string.h>
*/
import "C"

type BccModuleTables struct {
	Module       *bpf.Module
	Table        *bpf.Table
	TableName    string
	CPUFreqTable *bpf.Table
}

var (
	bccModule   *BccModuleTables = nil
	bccCounters                  = map[string]perfCounter{
		CPUCycleLabel:       {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CPU_CYCLES, true},
		CPURefCycleLabel:    {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_REF_CPU_CYCLES, true},
		CPUInstructionLabel: {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_INSTRUCTIONS, true},
		CacheMissLabel:      {unix.PERF_TYPE_HARDWARE, unix.PERF_COUNT_HW_CACHE_MISSES, true},
	}
)

var (
	// ebpfBatchGet is true if the kernel supports batch get operation
	ebpfBatchGet = true
	// ebpfBatchGetAndDelete is true if delete all the keys after batch get
	ebpfBatchGetAndDelete = ebpfBatchGet
)

const (
	BccBuilt = true
)

func loadBccModule(objProg []byte, options []string) (m *bpf.Module, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to attach the bpf program: %v", err)
			klog.Infoln(err)
			if m != nil {
				m.Close()
			}
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

	softirqEntry, err := m.LoadTracepoint("tracepoint__irq__softirq_entry")
	if err == nil {
		err = m.AttachTracepoint("irq:softirq_entry", softirqEntry)
		if err != nil {
			klog.Infof("failed to attach softirq_entry: %s", err)
		}
	} else {
		klog.Infof("failed to load softirq_entry: %s", err)
	}

	for arrayName, counter := range Counters {
		bpfPerfArrayName := arrayName + BpfPerfArrayPrefix
		t := bpf.NewTable(m.TableId(bpfPerfArrayName), m)
		if t == nil {
			return nil, fmt.Errorf("failed to find perf array: %s", bpfPerfArrayName)
		}
		perfErr := openPerfEvent(t, counter.EvType, counter.EvConfig)
		if perfErr != nil {
			// some hypervisors don't expose perf counters
			klog.Infof("failed to attach perf event %s: %v\n", bpfPerfArrayName, perfErr)
			counter.enabled = false

			// if any counter is not enabled, we need disable HardwareCountersEnabled
			HardwareCountersEnabled = false
		}
	}
	return m, nil
}

func attachBccModule() (*BccModuleTables, error) {
	bccModule = &BccModuleTables{}
	program := assets.Program
	objProg, err := assets.Asset(program)
	if err != nil {
		return nil, fmt.Errorf("failed to get program %q: %v", program, err)
	}
	// builtin runtime.NumCPU() returns the number of logical CPUs usable by the current process
	cores := runtime.NumCPU()
	if cpu, err := ghw.CPU(); err == nil {
		// we need to get the number of all CPUs,
		// so if /proc/cpuinfo is available, we can get the number of all CPUs
		cores = int(cpu.TotalThreads)
	}

	options := []string{
		"-DMAP_SIZE=" + strconv.Itoa(MapSize),
		"-DNUM_CPUS=" + strconv.Itoa(cores),
	}
	if config.EnabledEBPFCgroupID {
		options = append(options, "-DSET_GROUP_ID")
	}
	// TODO: verify if ebpf can run in the VM without hardware counter support, if not, we can disable the HC part and only collect the cpu time
	m, err := loadBccModule(objProg, options)
	if err != nil {
		klog.Infof("failed to attach perf module with options %v: %v, from default kernel source.\n", options, err)
		dirs := config.GetKernelSourceDirs()
		for _, dir := range dirs {
			klog.Infof("trying to load eBPF module with kernel source dir %s", dir)
			os.Setenv("BCC_KERNEL_SOURCE", dir)
			m, err = loadBccModule(objProg, options)
			if err != nil {
				klog.Infof("failed to attach perf module with options %v: %v, from kernel source %q\n", options, err, dir)
			} else {
				klog.Infof("Successfully loaded eBPF module with options: %v from kernel source %q", options, dir)
				break
			}
		}
	}
	if err != nil {
		klog.Infof("failed to attach perf module with options %v: %v, not able to load eBPF modules\n", options, err)
		return nil, fmt.Errorf("failed to attach perf module with options %v: %v, not able to load eBPF modules", options, err)
	}

	tableId := m.TableId(TableProcessName)
	table := bpf.NewTable(tableId, m)
	cpuFreqTable := bpf.NewTable(m.TableId(TableCPUFreqName), m)

	bccModule.Module = m
	bccModule.Table = table
	bccModule.TableName = TableProcessName
	bccModule.CPUFreqTable = cpuFreqTable

	klog.Infof("Successfully load eBPF module from bcc with option: %s", options)

	return bccModule, nil
}

func detachBccModule() {
	if bccModule != nil {
		closePerfEvent()
		if bccModule.Module != nil {
			bccModule.Module.Close()
			bccModule = nil
		}
	}
}

func bccCollectProcess() (processesData []ProcessBPFMetrics, err error) {
	processesData = []ProcessBPFMetrics{}
	if bccModule == nil {
		// nil error should be threw at attachment point, return empty data
		return
	}

	keysToDelete := [][]byte{}
	keys := [][]byte{}
	values := [][]byte{}

	var ct ProcessBPFMetrics
	// if ebpf map batch get operation is supported we use it
	if ebpfBatchGet {
		var batchGetErr error
		keys, values, batchGetErr = tableBatchGet(bccModule.Module, bccModule.TableName, uint32(unsafe.Sizeof(ct)), ebpfBatchGetAndDelete /* delete after get */)
		if batchGetErr != nil {
			klog.V(1).Infof("failed to get bpf table elements, err: %v", batchGetErr)
			ebpfBatchGet = false
			// if batch get is not supported we disable it for the next time
			ebpfBatchGetAndDelete = false
		}
	}
	// if ebpf map batch get operation is not supported we iterate over the map
	if !ebpfBatchGet {
		for it := bccModule.Table.Iter(); it.Next(); {
			key := it.Key()
			value := it.Leaf()
			keys = append(keys, key)
			values = append(values, value)
		}
	}

	// iterate over the keys and values
	for i := 0; i < len(keys); i++ {
		key := keys[i]
		data := values[i]
		err := binary.Read(bytes.NewBuffer(data), ByteOrder, &ct)
		if err != nil {
			klog.V(5).Infof("failed to decode received data: %v", err)
			continue // this only happens if there is a problem in the bpf code
		}
		// append ct data
		processesData = append(processesData, ct)
		// if not deleted during get, prepare the keys for delete
		if !ebpfBatchGetAndDelete {
			// if ebpf map batch deletion operation is supported we add the key to the list otherwise we delete the key
			if config.EnabledBPFBatchDelete {
				keysToDelete = append(keysToDelete, key)
			} else {
				err = bccModule.Table.Delete(key) // deleting the element to reset the counter values
				if err != nil && !os.IsNotExist(err) {
					klog.Infof("could not delete bpf table elements, err: %v", err)
				}
			}
		}
	}
	// if not deleted during get, delete it now
	if !ebpfBatchGetAndDelete {
		if config.EnabledBPFBatchDelete {
			err := tableDeleteBatch(bccModule.Module, bccModule.TableName, keysToDelete)
			// if the kernel does not support delete batch we delete all keys one by one
			if err != nil {
				bccModule.Table.DeleteAll()
				// if batch delete is not supported we disable it for the next time
				config.EnabledBPFBatchDelete = false
				klog.Infof("resetting EnabledBPFBatchDelete to %v", config.EnabledBPFBatchDelete)
			}
		}
	}
	return
}

func bccCollectFreq() (cpuFreqData map[int32]uint64, err error) {
	cpuFreqData = make(map[int32]uint64)
	if bccModule == nil {
		// nil error should be threw at attachment point, return empty data
		return
	}
	for it := bccModule.CPUFreqTable.Iter(); it.Next(); {
		cpu := int32(binary.LittleEndian.Uint32(it.Key()))
		freq := uint64(binary.LittleEndian.Uint32(it.Leaf()))
		cpuFreqData[cpu] = freq
	}
	return
}

///////////////////////////////////////////////////////////////////////////
// utility functions

func openPerfEvent(table *bpf.Table, typ, config int) error {
	perfKey := fmt.Sprintf("%d:%d", typ, config)
	if _, ok := PerfEvents[perfKey]; ok {
		return nil
	}

	cpus, err := cpuonline.Get()
	if err != nil {
		return fmt.Errorf("failed to determine online cpus: %v", err)
	}
	keySize := table.Config()["key_size"].(uint64)
	leafSize := table.Config()["leaf_size"].(uint64)

	if keySize != 4 || leafSize != 4 {
		return fmt.Errorf("passed table has wrong size: key_size=%d, leaf_size=%d", keySize, leafSize)
	}

	res := []int{}

	for _, i := range cpus {
		fd, err := C.bpf_open_perf_event(C.uint(typ), C.ulong(config), C.int(-1), C.int(i))
		if fd < 0 {
			return fmt.Errorf("failed to open bpf perf event: %v", err)
		}
		key := make([]byte, keySize)
		leaf := make([]byte, leafSize)
		ByteOrder.PutUint32(key, uint32(i))
		ByteOrder.PutUint32(leaf, uint32(fd))
		keyP := unsafe.Pointer(&key[0])
		leafP := unsafe.Pointer(&leaf[0])
		table.SetP(keyP, leafP)
		res = append(res, int(fd))
	}

	PerfEvents[perfKey] = res

	return nil
}

func closePerfEvent() {
	for _, vs := range PerfEvents {
		for _, v := range vs {
			C.bpf_close_perf_event_fd((C.int)(v))
		}
	}
	PerfEvents = map[string][]int{}
}

func tableDeleteBatch(module *bpf.Module, tableName string, keys [][]byte) error {
	// Allocate memory in C for the key pointers
	cKeys := C.malloc(C.size_t(len(keys)) * C.size_t(unsafe.Sizeof(uintptr(0))))
	defer C.free(cKeys)

	// Copy the key pointers from Go to C
	for i, key := range keys {
		ptr := C.malloc(C.size_t(len(key)))
		defer C.free(ptr)
		C.memcpy(ptr, unsafe.Pointer(&key[0]), C.size_t(len(key)))
		cKeyPtr := (**C.char)(unsafe.Pointer(uintptr(cKeys) + uintptr(i)*unsafe.Sizeof(uintptr(0))))
		*cKeyPtr = (*C.char)(ptr)
	}

	id := uint64(module.TableId(tableName))
	tableDesc := module.TableDesc(id)
	fd := C.int(tableDesc["fd"].(int))
	size := C.uint(len(keys))
	cKeysPtr := unsafe.Pointer(cKeys)

	_, err := C.bpf_delete_batch(fd, cKeysPtr, &size)
	klog.V(6).Infof("batch delete table %v size %v err: %v\n", fd, len(keys), err)
	// If the table is empty or keys are partially deleted, bpf_delete_batch will return errno ENOENT
	if err != nil {
		return err
	}
	return nil
}

// Batch get takes a key array and returns the value array or nil, and an 'ok' style indicator.
func tableBatchGet(mod *bpf.Module, tableName string, leafSize uint32, deleteAfterGet bool) ([][]byte, [][]byte, error) {
	id := uint64(mod.TableId(tableName))
	tableDesc := mod.TableDesc(id)
	tableId := tableDesc["fd"].(int)
	fd := C.int(tableId)
	// if setting entries to MapSize, lookup_batch will return -ENOENT and -1.
	// So we set entries to MapSize / leafSize
	entries := uint32(MapSize / leafSize)
	entriesInt := C.uint(entries)
	var (
		key  [][]byte
		leaf [][]byte
	)

	keySize := tableDesc["key_size"].(uint64)
	nextKey := C.uint(0)
	isEnd := false
	for !isEnd {
		keyArray := C.malloc(C.size_t(entries) * C.size_t(keySize))
		defer C.free(keyArray)

		leafArray := C.malloc(C.size_t(entries) * C.size_t(leafSize))
		defer C.free(leafArray)

		r, err := C.bpf_lookup_batch(fd, &nextKey, &nextKey, keyArray, leafArray, &(entriesInt))
		klog.V(6).Infof("batch get table %v ret: %v. requested/returned: %v/%v, err: %v\n", fd, r, entries, entriesInt, err)
		if err != nil {
			// os.IsNotExist means we reached the end of the table
			if os.IsNotExist(err) {
				isEnd = true
			} else {
				// r !=0 and other errors are unexpected
				if r != 0 {
					return key, leaf, fmt.Errorf("failed to batch get: %v", err)
				}
			}
		}
		for i := 0; i < int(entriesInt); i++ {
			k := C.GoBytes(unsafe.Pointer(uintptr(keyArray)+uintptr(i)*uintptr(keySize)), C.int(keySize))
			v := C.GoBytes(unsafe.Pointer(uintptr(leafArray)+uintptr(i)*uintptr(leafSize)), C.int(leafSize))
			key = append(key, k)
			leaf = append(leaf, v)
		}
		if int(entriesInt) > 0 && deleteAfterGet {
			r, err = C.bpf_delete_batch(fd, keyArray, &(entriesInt))
			klog.V(6).Infof("batch delete table %v ret: %v. requested/returned: %v/%v, err: %v\n", fd, r, entries, entriesInt, err)
			if r != 0 {
				return key, leaf, fmt.Errorf("failed to batch delete: %v", err)
			}
		}
	}
	klog.V(6).Infof("batch get table requested/returned: %v/%v\n", entries, len(key))
	return key, leaf, nil
}
