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

/*
 temporary placeholder till PR resolved
 https://github.com/iovisor/gobpf/pull/310
*/

import (
	"encoding/binary"
	"fmt"
	"os"
	"unsafe"

	bpf "github.com/iovisor/gobpf/bcc"
	"github.com/iovisor/gobpf/pkg/cpuonline"

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

var (
	perfEvents = map[string][]int{}
	byteOrder  binary.ByteOrder
)

func init() {
	byteOrder = bpf.GetHostByteOrder()
}

func openPerfEvent(table *bpf.Table, typ, config int) error {
	perfKey := fmt.Sprintf("%d:%d", typ, config)
	if _, ok := perfEvents[perfKey]; ok {
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
		byteOrder.PutUint32(key, uint32(i))
		byteOrder.PutUint32(leaf, uint32(fd))
		keyP := unsafe.Pointer(&key[0])
		leafP := unsafe.Pointer(&leaf[0])
		table.SetP(keyP, leafP)
		res = append(res, int(fd))
	}

	perfEvents[perfKey] = res

	return nil
}

func closePerfEvent() {
	for _, vs := range perfEvents {
		for _, v := range vs {
			C.bpf_close_perf_event_fd((C.int)(v))
		}
	}
}

func TableDeleteBatch(module *bpf.Module, tableName string, keys [][]byte) error {
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
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Batch get takes a key array and returns the value array or nil, and an 'ok' style indicator.
func TableBatchGet(mod *bpf.Module, tableName string, leafSize uint32, deleteAfterGet bool) ([][]byte, [][]byte, error) {
	id := uint64(mod.TableId(tableName))
	tableDesc := mod.TableDesc(id)
	tableId := tableDesc["fd"].(int)
	fd := C.int(tableId)
	// if setting entries to mapSize, lookup_batch will return -ENOENT and -1.
	// So we set entries to mapSize / leafSize
	entries := uint32(mapSize / leafSize)
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
			// r !=0 and os.IsNotExist means we reached the end of the table
			if os.IsNotExist(err) {
				isEnd = true
			} else {
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
