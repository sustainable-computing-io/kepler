package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

//
// BPFObjectIterator (Module Iterator)
//

// BPFObjectProgramIterator iterates over programs and maps in a BPF object
type BPFObjectIterator struct {
	m        *Module
	prevProg *BPFProg
	prevMap  *BPFMap
}

func (it *BPFObjectIterator) NextMap() *BPFMap {
	var startMapC *C.struct_bpf_map
	if it.prevMap != nil && it.prevMap.bpfMap != nil {
		startMapC = it.prevMap.bpfMap
	}

	bpfMapC, errno := C.bpf_object__next_map(it.m.obj, startMapC)
	if bpfMapC == nil {
		_ = errno // intentionally ignored
		return nil
	}

	bpfMap := &BPFMap{
		bpfMap: bpfMapC,
		module: it.m,
	}
	it.prevMap = bpfMap

	if !bpfMap.module.loaded {
		bpfMap.bpfMapLow = &BPFMapLow{
			fd:   -1,
			info: &BPFMapInfo{},
		}

		return bpfMap
	}

	fd := bpfMap.FileDescriptor()
	info, err := GetMapInfoByFD(fd)
	if err != nil {
		return nil
	}

	bpfMap.bpfMapLow = &BPFMapLow{
		fd:   fd,
		info: info,
	}

	return bpfMap
}

func (it *BPFObjectIterator) NextProgram() *BPFProg {
	var startProg *C.struct_bpf_program
	if it.prevProg != nil && it.prevProg.prog != nil {
		startProg = it.prevProg.prog
	}

	progC, errno := C.bpf_object__next_program(it.m.obj, startProg)
	if progC == nil {
		_ = errno // intentionally ignored
		return nil
	}

	prog := &BPFProg{
		prog:   progC,
		module: it.m,
	}

	it.prevProg = prog

	return prog
}
