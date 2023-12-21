package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

import (
	"fmt"
	"net"
	"syscall"
)

//
// TcHook
//

type TcHook struct {
	hook *C.struct_bpf_tc_hook
}

func (hook *TcHook) SetInterfaceByIndex(ifaceIdx int) {
	hook.hook.ifindex = C.int(ifaceIdx)
}

func (hook *TcHook) SetInterfaceByName(ifaceName string) error {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return err
	}
	hook.hook.ifindex = C.int(iface.Index)

	return nil
}

func (hook *TcHook) GetInterfaceIndex() int {
	return int(hook.hook.ifindex)
}

func (hook *TcHook) SetAttachPoint(attachPoint TcAttachPoint) {
	hook.hook.attach_point = uint32(attachPoint)
}

func (hook *TcHook) SetParent(a int, b int) {
	parent := (((a) << 16) & 0xFFFF0000) | ((b) & 0x0000FFFF)
	hook.hook.parent = C.uint(parent)
}

func (hook *TcHook) Create() error {
	retC := C.bpf_tc_hook_create(hook.hook)
	if retC < 0 {
		return fmt.Errorf("failed to create tc hook: %w", syscall.Errno(-retC))
	}

	return nil
}

func (hook *TcHook) Destroy() error {
	retC := C.bpf_tc_hook_destroy(hook.hook)
	if retC < 0 {
		return fmt.Errorf("failed to destroy tc hook: %w", syscall.Errno(-retC))
	}

	C.cgo_bpf_tc_hook_free(hook.hook)

	return nil
}

type TcOpts struct {
	ProgFd   int
	Flags    TcFlags
	ProgId   uint
	Handle   uint
	Priority uint
}

func tcOptsToC(tcOpts *TcOpts) (*C.struct_bpf_tc_opts, error) {
	if tcOpts == nil {
		return nil, nil
	}

	optsC, errno := C.cgo_bpf_tc_opts_new(
		C.int(tcOpts.ProgFd),
		C.uint(tcOpts.Flags),
		C.uint(tcOpts.ProgId),
		C.uint(tcOpts.Handle),
		C.uint(tcOpts.Priority),
	)
	if optsC == nil {
		return nil, fmt.Errorf("failed to create bpf_tc_opts: %w", errno)
	}

	return optsC, nil
}

func tcOptsFromC(tcOpts *TcOpts, optsC *C.struct_bpf_tc_opts) {
	if optsC == nil {
		return
	}

	tcOpts.ProgFd = int(C.cgo_bpf_tc_opts_prog_fd(optsC))
	tcOpts.Flags = TcFlags(C.cgo_bpf_tc_opts_flags(optsC))
	tcOpts.ProgId = uint(C.cgo_bpf_tc_opts_prog_id(optsC))
	tcOpts.Handle = uint(C.cgo_bpf_tc_opts_handle(optsC))
	tcOpts.Priority = uint(C.cgo_bpf_tc_opts_priority(optsC))
}

func (hook *TcHook) Attach(tcOpts *TcOpts) error {
	optsC, err := tcOptsToC(tcOpts)
	if err != nil {
		return err
	}
	defer C.cgo_bpf_tc_opts_free(optsC)

	retC := C.bpf_tc_attach(hook.hook, optsC)
	if retC < 0 {
		return fmt.Errorf("failed to attach tc hook: %w", syscall.Errno(-retC))
	}

	// update tcOpts with the values from the libbpf
	tcOptsFromC(tcOpts, optsC)

	return nil
}

func (hook *TcHook) Detach(tcOpts *TcOpts) error {
	optsC, err := tcOptsToC(tcOpts)
	if err != nil {
		return err
	}
	defer C.cgo_bpf_tc_opts_free(optsC)

	retC := C.bpf_tc_detach(hook.hook, optsC)
	if retC < 0 {
		return fmt.Errorf("failed to detach tc hook: %w", syscall.Errno(-retC))
	}

	// update tcOpts with the values from the libbpf
	tcOptsFromC(tcOpts, optsC)

	return nil
}

func (hook *TcHook) Query(tcOpts *TcOpts) error {
	optsC, err := tcOptsToC(tcOpts)
	if err != nil {
		return err
	}
	defer C.cgo_bpf_tc_opts_free(optsC)

	retC := C.bpf_tc_query(hook.hook, optsC)
	if retC < 0 {
		return fmt.Errorf("failed to query tc hook: %w", syscall.Errno(-retC))
	}

	// update tcOpts with the values from the libbpf
	tcOptsFromC(tcOpts, optsC)

	return nil
}
