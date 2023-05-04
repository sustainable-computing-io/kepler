// Copyright 2023 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bcc

import (
	"fmt"
	"os"
	"unsafe"
)

/*
#cgo CFLAGS: -I/usr/include/bcc/compat
#cgo LDFLAGS: -lbcc
#include <linux/bpf.h>
#include <bcc/bcc_common.h>
#include <bcc/libbpf.h>
*/
import "C"

var (
	printResult = os.Getenv("BCC_PRINT_RESULT") != ""
)

// Batch get takes a key array and returns the value array or nil, and an 'ok' style indicator.
func (table *Table) BatchGet(leafSize uint32, deleteAfterGet bool) ([][]byte, [][]byte, error) {
	mod := table.module.p
	fd := C.bpf_table_fd_id(mod, table.id)
	// if setting entries to max entries, lookup_batch will return -ENOENT and -1.
	// So we set entries to max entries / leafSize
	entries := uint32(uint32(C.bpf_table_max_entries_id(mod, table.id)) / leafSize)
	entriesInt := C.uint(entries)
	var (
		key  [][]byte
		leaf [][]byte
	)

	keySize := C.bpf_table_key_size_id(mod, table.id)
	nextKey := C.uint(0)
	isEnd := false
	for !isEnd {
		keyArray := C.malloc(C.size_t(entries) * C.size_t(keySize))
		defer C.free(keyArray)

		leafArray := C.malloc(C.size_t(entries) * C.size_t(leafSize))
		defer C.free(leafArray)

		r, err := C.bpf_lookup_batch(fd, &nextKey, &nextKey, keyArray, leafArray, &(entriesInt))
		if printResult {
			fmt.Printf("batch get table %v ret: %v. requested/returned: %v/%v, err: %v\n", fd, r, entries, entriesInt, err)
		}
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
			if printResult {
				fmt.Printf("batch delete table %v ret: %v. requested/returned: %v/%v, err: %v\n", fd, r, entries, entriesInt, err)
			}
			if r != 0 {
				return key, leaf, fmt.Errorf("failed to batch delete: %v", err)
			}
		}
	}
	if printResult {
		fmt.Printf("batch get table requested/returned: %v/%v\n", entries, len(key))
	}
	return key, leaf, nil
}
