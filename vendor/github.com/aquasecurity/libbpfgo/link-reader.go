package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

import "syscall"

//
// BPFLinkReader
//

// BPFLinkReader read data from a BPF link
type BPFLinkReader struct {
	l  *BPFLink
	fd int
}

func (i *BPFLinkReader) Read(p []byte) (n int, err error) {
	return syscall.Read(i.fd, p)
}

func (i *BPFLinkReader) Close() error {
	return syscall.Close(i.fd)
}
