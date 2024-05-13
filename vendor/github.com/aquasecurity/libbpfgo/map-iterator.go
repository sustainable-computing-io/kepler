package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

import (
	"syscall"
	"unsafe"
)

//
// BPFMapIterator (low-level API)
//

// BPFMapIterator iterates over keys in a BPF map.
type BPFMapIterator struct {
	mapFD   int
	keySize int
	err     error
	prev    []byte
	next    []byte
}

// Next advances the iterator to the next key in the map.
func (it *BPFMapIterator) Next() bool {
	if it.err != nil {
		return false
	}

	prevPtr := unsafe.Pointer(nil)
	if it.next != nil {
		prevPtr = unsafe.Pointer(&it.next[0])
	}

	next := make([]byte, it.keySize)
	nextPtr := unsafe.Pointer(&next[0])

	retC := C.bpf_map_get_next_key(C.int(it.mapFD), prevPtr, nextPtr)
	if retC < 0 {
		if err := syscall.Errno(-retC); err != syscall.ENOENT {
			it.err = err
		}

		return false
	}

	it.prev = it.next
	it.next = next

	return true
}

// Key returns the current key value of the iterator, if the most recent call
// to Next returned true.
// The slice is valid only until the next call to Next.
func (it *BPFMapIterator) Key() []byte {
	return it.next
}

// Err returns the last error that ocurred while table.Iter or iter.Next.
func (it *BPFMapIterator) Err() error {
	return it.err
}
