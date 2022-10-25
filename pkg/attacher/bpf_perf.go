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
	"unsafe"

	bpf "github.com/iovisor/gobpf/bcc"
	"github.com/iovisor/gobpf/pkg/cpuonline"
)

/*
#cgo CFLAGS: -I/usr/include/bcc/compat
#cgo LDFLAGS: -lbcc
#include <bcc/bcc_common.h>
#include <bcc/libbpf.h>
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
		return fmt.Errorf("passed table has wrong size")
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
