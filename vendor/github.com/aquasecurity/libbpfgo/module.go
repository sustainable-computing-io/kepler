package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

//
// Module (BPF Object)
//

type Module struct {
	obj      *C.struct_bpf_object
	links    []*BPFLink
	perfBufs []*PerfBuffer
	ringBufs []*RingBuffer
	elf      *elf.File
	loaded   bool
}

//
// New Module Helpers
//

type NewModuleArgs struct {
	KConfigFilePath string
	BTFObjPath      string
	BPFObjName      string
	BPFObjPath      string
	BPFObjBuff      []byte
	SkipMemlockBump bool
	KernelLogLevel  uint32
}

func NewModuleFromFile(bpfObjPath string) (*Module, error) {
	return NewModuleFromFileArgs(NewModuleArgs{
		BPFObjPath: bpfObjPath,
	})
}

func NewModuleFromFileArgs(args NewModuleArgs) (*Module, error) {
	f, err := elf.Open(args.BPFObjPath)
	if err != nil {
		return nil, err
	}
	C.cgo_libbpf_set_print_fn()

	// If skipped, we rely on libbpf to do the bumping if deemed necessary
	if !args.SkipMemlockBump {
		// TODO: remove this once libbpf memory limit bump issue is solved
		if err := bumpMemlockRlimit(); err != nil {
			return nil, err
		}
	}

	var btfFilePathC *C.char
	var kconfigPathC *C.char

	// instruct libbpf to use user provided kernel BTF file
	if args.BTFObjPath != "" {
		btfFilePathC = C.CString(args.BTFObjPath)
		defer C.free(unsafe.Pointer(btfFilePathC))
	}
	// instruct libbpf to use user provided KConfigFile
	if args.KConfigFilePath != "" {
		kconfigPathC = C.CString(args.KConfigFilePath)
		defer C.free(unsafe.Pointer(kconfigPathC))
	}

	kernelLogLevelC := C.uint(args.KernelLogLevel)

	optsC, errno := C.cgo_bpf_object_open_opts_new(btfFilePathC, kconfigPathC, nil, kernelLogLevelC)
	if optsC == nil {
		return nil, fmt.Errorf("failed to create bpf_object_open_opts: %w", errno)
	}
	defer C.cgo_bpf_object_open_opts_free(optsC)

	bpfFileC := C.CString(args.BPFObjPath)
	defer C.free(unsafe.Pointer(bpfFileC))

	objC, errno := C.bpf_object__open_file(bpfFileC, optsC)
	if objC == nil {
		return nil, fmt.Errorf("failed to open BPF object at path %s: %w", args.BPFObjPath, errno)
	}

	return &Module{
		obj: objC,
		elf: f,
	}, nil
}

func NewModuleFromBuffer(bpfObjBuff []byte, bpfObjName string) (*Module, error) {
	return NewModuleFromBufferArgs(NewModuleArgs{
		BPFObjBuff: bpfObjBuff,
		BPFObjName: bpfObjName,
	})
}

func NewModuleFromBufferArgs(args NewModuleArgs) (*Module, error) {
	f, err := elf.NewFile(bytes.NewReader(args.BPFObjBuff))
	if err != nil {
		return nil, err
	}
	C.cgo_libbpf_set_print_fn()

	// TODO: remove this once libbpf memory limit bump issue is solved
	if err := bumpMemlockRlimit(); err != nil {
		return nil, err
	}

	if args.BTFObjPath == "" {
		args.BTFObjPath = "/sys/kernel/btf/vmlinux"
	}

	btfFilePathC := C.CString(args.BTFObjPath)
	defer C.free(unsafe.Pointer(btfFilePathC))
	kConfigPathC := C.CString(args.KConfigFilePath)
	defer C.free(unsafe.Pointer(kConfigPathC))
	bpfObjNameC := C.CString(args.BPFObjName)
	defer C.free(unsafe.Pointer(bpfObjNameC))
	bpfBuffC := unsafe.Pointer(C.CBytes(args.BPFObjBuff))
	defer C.free(bpfBuffC)
	bpfBuffSizeC := C.size_t(len(args.BPFObjBuff))
	kernelLogLevelC := C.uint(args.KernelLogLevel)

	if len(args.KConfigFilePath) <= 2 {
		kConfigPathC = nil
	}

	optsC, errno := C.cgo_bpf_object_open_opts_new(btfFilePathC, kConfigPathC, bpfObjNameC, kernelLogLevelC)
	if optsC == nil {
		return nil, fmt.Errorf("failed to create bpf_object_open_opts: %w", errno)
	}
	defer C.cgo_bpf_object_open_opts_free(optsC)

	objC, errno := C.bpf_object__open_mem(bpfBuffC, bpfBuffSizeC, optsC)
	if objC == nil {
		return nil, fmt.Errorf("failed to open BPF object %s: %w", args.BPFObjName, errno)
	}

	return &Module{
		obj: objC,
		elf: f,
	}, nil
}

// NOTE: libbpf has started raising limits by default but, unfortunately, that
// seems to be failing in current libbpf version. The memory limit bump might be
// removed once this is sorted out.
func bumpMemlockRlimit() error {
	var rLimit syscall.Rlimit
	rLimit.Max = 512 << 20 /* 512 MBs */
	rLimit.Cur = 512 << 20 /* 512 MBs */
	err := syscall.Setrlimit(C.RLIMIT_MEMLOCK, &rLimit)
	if err != nil {
		return fmt.Errorf("error setting rlimit: %v", err)
	}
	return nil
}

//
// Module Methods
//

func (m *Module) Close() {
	for _, pb := range m.perfBufs {
		pb.Close()
	}
	for _, rb := range m.ringBufs {
		rb.Close()
	}
	for _, link := range m.links {
		if link.link != nil {
			link.Destroy()
		}
	}
	C.bpf_object__close(m.obj)
}

func (m *Module) BPFLoadObject() error {
	retC := C.bpf_object__load(m.obj)
	if retC < 0 {
		return fmt.Errorf("failed to load BPF object: %w", syscall.Errno(-retC))
	}
	m.loaded = true
	m.elf.Close()

	return nil
}

// InitGlobalVariable sets global variables (defined in .data or .rodata)
// in bpf code. It must be called before the BPF object is loaded.
func (m *Module) InitGlobalVariable(name string, value interface{}) error {
	if m.loaded {
		return errors.New("must be called before the BPF object is loaded")
	}
	s, err := getGlobalVariableSymbol(m.elf, name)
	if err != nil {
		return err
	}
	bpfMap, err := m.GetMap(s.sectionName)
	if err != nil {
		return err
	}

	// get current value
	currMapValue, err := bpfMap.InitialValue()
	if err != nil {
		return err
	}

	// generate new value
	newMapValue := make([]byte, bpfMap.ValueSize())
	copy(newMapValue, currMapValue)
	data := bytes.NewBuffer(nil)
	if err := binary.Write(data, s.byteOrder, value); err != nil {
		return err
	}
	varValue := data.Bytes()
	start := s.offset
	end := s.offset + len(varValue)
	if len(varValue) > s.size || end > bpfMap.ValueSize() {
		return errors.New("invalid value")
	}
	copy(newMapValue[start:end], varValue)

	// save new value
	err = bpfMap.SetInitialValue(unsafe.Pointer(&newMapValue[0]))
	return err
}

func (m *Module) GetMap(mapName string) (*BPFMap, error) {
	mapNameC := C.CString(mapName)
	defer C.free(unsafe.Pointer(mapNameC))

	bpfMapC, errno := C.bpf_object__find_map_by_name(m.obj, mapNameC)
	if bpfMapC == nil {
		return nil, fmt.Errorf("failed to find BPF map %s: %w", mapName, errno)
	}

	bpfMap := &BPFMap{
		bpfMap: bpfMapC,
		module: m,
	}

	if !m.loaded {
		bpfMap.bpfMapLow = &BPFMapLow{
			fd:   -1,
			info: &BPFMapInfo{},
		}

		return bpfMap, nil
	}

	fd := bpfMap.FileDescriptor()
	info, err := GetMapInfoByFD(fd)
	if err != nil {
		// Compatibility Note: Some older kernels lack BTF (BPF Type Format)
		// support for specific BPF map types. In such scenarios, libbpf may
		// fail (EPERM) when attempting to retrieve information for these maps.
		// Reference: https://elixir.bootlin.com/linux/v5.15.75/source/tools/lib/bpf/gen_loader.c#L401
		//
		// However, we can still get some map info from the BPF map high level API.
		bpfMap.bpfMapLow = &BPFMapLow{
			fd: fd,
			info: &BPFMapInfo{
				Type:                  bpfMap.Type(),
				ID:                    0,
				KeySize:               uint32(bpfMap.KeySize()),
				ValueSize:             uint32(bpfMap.ValueSize()),
				MaxEntries:            bpfMap.MaxEntries(),
				MapFlags:              uint32(bpfMap.MapFlags()),
				Name:                  bpfMap.Name(),
				IfIndex:               bpfMap.IfIndex(),
				BTFVmlinuxValueTypeID: 0,
				NetnsDev:              0,
				NetnsIno:              0,
				BTFID:                 0,
				BTFKeyTypeID:          0,
				BTFValueTypeID:        0,
				MapExtra:              bpfMap.MapExtra(),
			},
		}

		return bpfMap, nil
	}

	bpfMap.bpfMapLow = &BPFMapLow{
		fd:   fd,
		info: info,
	}

	return bpfMap, nil
}

func (m *Module) GetProgram(progName string) (*BPFProg, error) {
	progNameC := C.CString(progName)
	defer C.free(unsafe.Pointer(progNameC))

	progC, errno := C.bpf_object__find_program_by_name(m.obj, progNameC)
	if progC == nil {
		return nil, fmt.Errorf("failed to find BPF program %s: %w", progName, errno)
	}

	return &BPFProg{
		prog:   progC,
		module: m,
	}, nil
}

func (m *Module) InitRingBuf(mapName string, eventsChan chan []byte) (*RingBuffer, error) {
	bpfMap, err := m.GetMap(mapName)
	if err != nil {
		return nil, err
	}

	if eventsChan == nil {
		return nil, fmt.Errorf("events channel can not be nil")
	}

	slot := eventChannels.put(eventsChan)
	if slot == -1 {
		return nil, fmt.Errorf("max ring buffers reached")
	}

	rbC, errno := C.cgo_init_ring_buf(C.int(bpfMap.FileDescriptor()), C.uintptr_t(slot))
	if rbC == nil {
		return nil, fmt.Errorf("failed to initialize ring buffer: %w", errno)
	}

	ringBuf := &RingBuffer{
		rb:     rbC,
		bpfMap: bpfMap,
		slot:   uint(slot),
	}
	m.ringBufs = append(m.ringBufs, ringBuf)
	return ringBuf, nil
}

func (m *Module) InitPerfBuf(mapName string, eventsChan chan []byte, lostChan chan uint64, pageCnt int) (*PerfBuffer, error) {
	bpfMap, err := m.GetMap(mapName)
	if err != nil {
		return nil, fmt.Errorf("failed to init perf buffer: %v", err)
	}

	if eventsChan == nil {
		return nil, fmt.Errorf("failed to init perf buffer: events channel can not be nil")
	}

	perfBuf := &PerfBuffer{
		bpfMap:     bpfMap,
		eventsChan: eventsChan,
		lostChan:   lostChan,
	}

	slot := eventChannels.put(perfBuf)
	if slot == -1 {
		return nil, fmt.Errorf("max number of ring/perf buffers reached")
	}

	pbC, errno := C.cgo_init_perf_buf(C.int(bpfMap.FileDescriptor()), C.int(pageCnt), C.uintptr_t(slot))
	if pbC == nil {
		eventChannels.remove(uint(slot))
		return nil, fmt.Errorf("failed to initialize perf buffer: %w", errno)
	}

	perfBuf.pb = pbC
	perfBuf.slot = uint(slot)

	m.perfBufs = append(m.perfBufs, perfBuf)
	return perfBuf, nil
}

func (m *Module) TcHookInit() *TcHook {
	return &TcHook{
		hook: C.cgo_bpf_tc_hook_new(),
	}
}

func (m *Module) Iterator() *BPFObjectIterator {
	return &BPFObjectIterator{
		m:        m,
		prevProg: nil,
		prevMap:  nil,
	}
}

func (m *Module) linkExist(prog *BPFProg) bool {
	for _, link := range m.links {
		if link.prog.Name() == prog.Name() {
			return true
		}
	}

	return false
}

// AttachPrograms attach all loaded and no attached progs once like bpf_object__attach_skeleton
func (m *Module) AttachPrograms() error {
	iters := m.Iterator()
	for {
		prog := iters.NextProgram()
		if prog == nil {
			break
		}

		if !prog.Autoload() || !prog.Autoattach() {
			continue
		}
		// if link already exist (is attached), skip it
		if m.linkExist(prog) {
			continue
		}

		link, err := prog.AttachGeneric()
		if err != nil {
			return err
		}

		m.links = append(m.links, link)
	}

	return nil
}

// DetachPrograms detach all attached progs once like bpf_object__detach_skeleton
func (m *Module) DetachPrograms() error {
	errInfo := make(map[string]error)

	for _, link := range m.links {
		err := link.Destroy()
		if err != nil {
			errInfo[link.prog.Name()] = err
		}
	}
	m.links = nil

	if len(errInfo) > 0 {
		var str string
		for name, err := range errInfo {
			str += fmt.Sprintf(" [prog:%s,err:%s]", name, err)
		}

		return fmt.Errorf("link destroy failed:%s", str)
	}

	return nil
}
