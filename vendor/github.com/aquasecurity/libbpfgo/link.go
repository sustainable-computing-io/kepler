package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

import (
	"fmt"
	"syscall"
	"unsafe"
)

//
// LinkType
//

type LinkType int

const (
	Tracepoint LinkType = iota
	RawTracepoint
	Kprobe
	Kretprobe
	LSM
	PerfEvent
	Uprobe
	Uretprobe
	Tracing
	XDP
	Cgroup
	CgroupLegacy
	Netns
	Iter
)

//
// BPFLink
//

type bpfLinkLegacy struct {
	attachType BPFAttachType
	cgroupDir  string
}

type BPFLink struct {
	link      *C.struct_bpf_link
	prog      *BPFProg
	linkType  LinkType
	eventName string
	legacy    *bpfLinkLegacy // if set, this is a fake BPFLink
}

func (l *BPFLink) DestroyLegacy(linkType LinkType) error {
	switch l.linkType {
	case CgroupLegacy:
		return l.prog.DetachCgroupLegacy(
			l.legacy.cgroupDir,
			l.legacy.attachType,
		)
	}

	return fmt.Errorf("unable to destroy legacy link")
}

func (l *BPFLink) Destroy() error {
	if l.legacy != nil {
		return l.DestroyLegacy(l.linkType)
	}
	if retC := C.bpf_link__destroy(l.link); retC < 0 {
		return syscall.Errno(-retC)
	}

	l.link = nil

	return nil
}

func (l *BPFLink) FileDescriptor() int {
	return int(C.bpf_link__fd(l.link))
}

// Deprecated: use BPFLink.FileDescriptor() instead.
func (l *BPFLink) GetFd() int {
	return l.FileDescriptor()
}

func (l *BPFLink) Pin(pinPath string) error {
	pathC := C.CString(pinPath)
	defer C.free(unsafe.Pointer(pathC))

	retC := C.bpf_link__pin(l.link, pathC)
	if retC < 0 {
		return fmt.Errorf("failed to pin link %s to path %s: %w", l.eventName, pinPath, syscall.Errno(-retC))
	}

	return nil
}

func (l *BPFLink) Unpin() error {
	retC := C.bpf_link__unpin(l.link)
	if retC < 0 {
		return fmt.Errorf("failed to unpin link %s: %w", l.eventName, syscall.Errno(-retC))
	}

	return nil
}

//
// BPF Link Reader (low-level)
//

func (l *BPFLink) Reader() (*BPFLinkReader, error) {
	fdC := C.bpf_iter_create(C.int(l.FileDescriptor()))
	if fdC < 0 {
		return nil, fmt.Errorf("failed to create reader: %w", syscall.Errno(-fdC))
	}

	return &BPFLinkReader{
		l:  l,
		fd: int(fdC),
	}, nil
}
