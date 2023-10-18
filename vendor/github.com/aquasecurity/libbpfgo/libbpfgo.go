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
	"net"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const (
	// Maximum number of channels (RingBuffers + PerfBuffers) supported
	maxEventChannels = 512
)

// MajorVersion returns the major semver version of libbpf.
func MajorVersion() int {
	return C.LIBBPF_MAJOR_VERSION
}

// MinorVersion returns the minor semver version of libbpf.
func MinorVersion() int {
	return C.LIBBPF_MINOR_VERSION
}

// LibbpfVersionString returns the string representation of the libbpf version which
// libbpfgo is linked against
func LibbpfVersionString() string {
	return fmt.Sprintf("v%d.%d", MajorVersion(), MinorVersion())
}

type Module struct {
	obj      *C.struct_bpf_object
	links    []*BPFLink
	perfBufs []*PerfBuffer
	ringBufs []*RingBuffer
	elf      *elf.File
	loaded   bool
}

type BPFProg struct {
	name       string
	prog       *C.struct_bpf_program
	module     *Module
	pinnedPath string
}

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

type BPFLinkLegacy struct {
	attachType BPFAttachType
	cgroupDir  string
}

type BPFLink struct {
	link      *C.struct_bpf_link
	prog      *BPFProg
	linkType  LinkType
	eventName string
	legacy    *BPFLinkLegacy // if set, this is a fake BPFLink
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
	if ret := C.bpf_link__destroy(l.link); ret < 0 {
		return syscall.Errno(-ret)
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
	path := C.CString(pinPath)
	errC := C.bpf_link__pin(l.link, path)
	C.free(unsafe.Pointer(path))
	if errC != 0 {
		return fmt.Errorf("failed to pin link %s to path %s: %w", l.eventName, pinPath, syscall.Errno(-errC))
	}
	return nil
}

func (l *BPFLink) Unpin(pinPath string) error {
	path := C.CString(pinPath)
	errC := C.bpf_link__unpin(l.link)
	C.free(unsafe.Pointer(path))
	if errC != 0 {
		return fmt.Errorf("failed to unpin link %s from path %s: %w", l.eventName, pinPath, syscall.Errno(-errC))
	}
	return nil
}

type PerfBuffer struct {
	pb         *C.struct_perf_buffer
	bpfMap     *BPFMap
	slot       uint
	eventsChan chan []byte
	lostChan   chan uint64
	stop       chan struct{}
	closed     bool
	wg         sync.WaitGroup
}

type RingBuffer struct {
	rb     *C.struct_ring_buffer
	bpfMap *BPFMap
	slot   uint
	stop   chan struct{}
	closed bool
	wg     sync.WaitGroup
}

type NewModuleArgs struct {
	KConfigFilePath string
	BTFObjPath      string
	BPFObjName      string
	BPFObjPath      string
	BPFObjBuff      []byte
	SkipMemlockBump bool
}

func NewModuleFromFile(bpfObjPath string) (*Module, error) {
	return NewModuleFromFileArgs(NewModuleArgs{
		BPFObjPath: bpfObjPath,
	})
}

// LibbpfStrictMode is an enum as defined in https://github.com/libbpf/libbpf/blob/2cd2d03f63242c048a896179398c68d2dbefe3d6/src/libbpf_legacy.h#L23
type LibbpfStrictMode uint32

const (
	LibbpfStrictModeAll               LibbpfStrictMode = C.LIBBPF_STRICT_ALL
	LibbpfStrictModeNone              LibbpfStrictMode = C.LIBBPF_STRICT_NONE
	LibbpfStrictModeCleanPtrs         LibbpfStrictMode = C.LIBBPF_STRICT_CLEAN_PTRS
	LibbpfStrictModeDirectErrs        LibbpfStrictMode = C.LIBBPF_STRICT_DIRECT_ERRS
	LibbpfStrictModeSecName           LibbpfStrictMode = C.LIBBPF_STRICT_SEC_NAME
	LibbpfStrictModeNoObjectList      LibbpfStrictMode = C.LIBBPF_STRICT_NO_OBJECT_LIST
	LibbpfStrictModeAutoRlimitMemlock LibbpfStrictMode = C.LIBBPF_STRICT_AUTO_RLIMIT_MEMLOCK
	LibbpfStrictModeMapDefinitions    LibbpfStrictMode = C.LIBBPF_STRICT_MAP_DEFINITIONS
)

func (b LibbpfStrictMode) String() (str string) {
	x := map[LibbpfStrictMode]string{
		LibbpfStrictModeAll:               "LIBBPF_STRICT_ALL",
		LibbpfStrictModeNone:              "LIBBPF_STRICT_NONE",
		LibbpfStrictModeCleanPtrs:         "LIBBPF_STRICT_CLEAN_PTRS",
		LibbpfStrictModeDirectErrs:        "LIBBPF_STRICT_DIRECT_ERRS",
		LibbpfStrictModeSecName:           "LIBBPF_STRICT_SEC_NAME",
		LibbpfStrictModeNoObjectList:      "LIBBPF_STRICT_NO_OBJECT_LIST",
		LibbpfStrictModeAutoRlimitMemlock: "LIBBPF_STRICT_AUTO_RLIMIT_MEMLOCK",
		LibbpfStrictModeMapDefinitions:    "LIBBPF_STRICT_MAP_DEFINITIONS",
	}

	str, ok := x[b]
	if !ok {
		str = LibbpfStrictModeNone.String()
	}
	return str
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

func SetStrictMode(mode LibbpfStrictMode) {
	C.libbpf_set_strict_mode(uint32(mode))
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

	opts := C.struct_bpf_object_open_opts{}
	opts.sz = C.sizeof_struct_bpf_object_open_opts

	bpfFile := C.CString(args.BPFObjPath)
	defer C.free(unsafe.Pointer(bpfFile))

	// instruct libbpf to use user provided kernel BTF file
	if args.BTFObjPath != "" {
		btfFile := C.CString(args.BTFObjPath)
		opts.btf_custom_path = btfFile
		defer C.free(unsafe.Pointer(btfFile))
	}

	// instruct libbpf to use user provided KConfigFile
	if args.KConfigFilePath != "" {
		kConfigFile := C.CString(args.KConfigFilePath)
		opts.kconfig = kConfigFile
		defer C.free(unsafe.Pointer(kConfigFile))
	}

	obj, errno := C.bpf_object__open_file(bpfFile, &opts)
	if obj == nil {
		return nil, fmt.Errorf("failed to open BPF object at path %s: %w", args.BPFObjPath, errno)
	}

	return &Module{
		obj: obj,
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

	cBTFFilePath := C.CString(args.BTFObjPath)
	defer C.free(unsafe.Pointer(cBTFFilePath))
	cKconfigPath := C.CString(args.KConfigFilePath)
	defer C.free(unsafe.Pointer(cKconfigPath))
	cBPFObjName := C.CString(args.BPFObjName)
	defer C.free(unsafe.Pointer(cBPFObjName))
	cBPFBuff := unsafe.Pointer(C.CBytes(args.BPFObjBuff))
	defer C.free(cBPFBuff)
	cBPFBuffSize := C.size_t(len(args.BPFObjBuff))

	if len(args.KConfigFilePath) <= 2 {
		cKconfigPath = nil
	}

	cOpts, errno := C.cgo_bpf_object_open_opts_new(cBTFFilePath, cKconfigPath, cBPFObjName)
	if cOpts == nil {
		return nil, fmt.Errorf("failed to create bpf_object_open_opts to %s: %w", args.BPFObjName, errno)
	}
	defer C.cgo_bpf_object_open_opts_free(cOpts)

	obj, errno := C.bpf_object__open_mem(cBPFBuff, cBPFBuffSize, cOpts)
	if obj == nil {
		return nil, fmt.Errorf("failed to open BPF object %s: %w", args.BPFObjName, errno)
	}

	return &Module{
		obj: obj,
		elf: f,
	}, nil
}

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
	ret := C.bpf_object__load(m.obj)
	if ret != 0 {
		return fmt.Errorf("failed to load BPF object: %w", syscall.Errno(-ret))
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
	cs := C.CString(mapName)
	bpfMapC, errno := C.bpf_object__find_map_by_name(m.obj, cs)
	C.free(unsafe.Pointer(cs))
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
		// The consumer of this API may not have sufficient privileges to get
		// map info via BPF syscall. However, "some" map info still can be
		// retrieved from the BPF object itself.
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

// BPFObjectProgramIterator iterates over maps in a BPF object
type BPFObjectIterator struct {
	m        *Module
	prevProg *BPFProg
	prevMap  *BPFMap
}

func (m *Module) Iterator() *BPFObjectIterator {
	return &BPFObjectIterator{
		m:        m,
		prevProg: nil,
		prevMap:  nil,
	}
}

func (it *BPFObjectIterator) NextMap() *BPFMap {
	var startMap *C.struct_bpf_map
	if it.prevMap != nil && it.prevMap.bpfMap != nil {
		startMap = it.prevMap.bpfMap
	}

	m := C.bpf_object__next_map(it.m.obj, startMap)
	if m == nil {
		return nil
	}

	bpfMap := &BPFMap{
		bpfMap: m,
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

	p := C.bpf_object__next_program(it.m.obj, startProg)
	if p == nil {
		return nil
	}
	cName := C.bpf_program__name(p)

	prog := &BPFProg{
		name:   C.GoString(cName),
		prog:   p,
		module: it.m,
	}
	it.prevProg = prog
	return prog
}

// BPFLinkReader read data from a BPF link
type BPFLinkReader struct {
	l  *BPFLink
	fd int
}

func (l *BPFLink) Reader() (*BPFLinkReader, error) {
	fd, errno := C.bpf_iter_create(C.int(l.FileDescriptor()))
	if fd < 0 {
		return nil, fmt.Errorf("failed to create reader: %w", errno)
	}
	return &BPFLinkReader{
		l:  l,
		fd: int(uintptr(fd)),
	}, nil
}

func (i *BPFLinkReader) Read(p []byte) (n int, err error) {
	return syscall.Read(i.fd, p)
}

func (i *BPFLinkReader) Close() error {
	return syscall.Close(i.fd)
}

func (m *Module) GetProgram(progName string) (*BPFProg, error) {
	cs := C.CString(progName)
	prog, errno := C.bpf_object__find_program_by_name(m.obj, cs)
	C.free(unsafe.Pointer(cs))
	if prog == nil {
		return nil, fmt.Errorf("failed to find BPF program %s: %w", progName, errno)
	}

	return &BPFProg{
		name:   progName,
		prog:   prog,
		module: m,
	}, nil
}

func (p *BPFProg) FileDescriptor() int {
	return int(C.bpf_program__fd(p.prog))
}

// Deprecated: use BPFProg.FileDescriptor() instead.
func (p *BPFProg) GetFd() int {
	return p.FileDescriptor()
}

func (p *BPFProg) Pin(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %s: %v", path, err)
	}

	cs := C.CString(absPath)
	ret := C.bpf_program__pin(p.prog, cs)
	C.free(unsafe.Pointer(cs))
	if ret != 0 {
		return fmt.Errorf("failed to pin program %s to %s: %w", p.name, path, syscall.Errno(-ret))
	}
	p.pinnedPath = absPath
	return nil
}

func (p *BPFProg) Unpin(path string) error {
	cs := C.CString(path)
	ret := C.bpf_program__unpin(p.prog, cs)
	C.free(unsafe.Pointer(cs))
	if ret != 0 {
		return fmt.Errorf("failed to unpin program %s to %s: %w", p.name, path, syscall.Errno(-ret))
	}
	p.pinnedPath = ""
	return nil
}

func (p *BPFProg) GetModule() *Module {
	return p.module
}

func (p *BPFProg) Name() string {
	return C.GoString(C.bpf_program__name(p.prog))
}

// Deprecated: use BPFProg.Name() instead.
func (p *BPFProg) GetName() string {
	return p.Name()
}

func (p *BPFProg) SectionName() string {
	return C.GoString(C.bpf_program__section_name(p.prog))
}

// Deprecated: use BPFProg.SectionName() instead.
func (p *BPFProg) GetSectionName() string {
	return p.SectionName()
}

func (p *BPFProg) PinPath() string {
	return p.pinnedPath // There's no LIBBPF_API for bpf program
}

// Deprecated: use BPFProg.PinPath() instead.
func (p *BPFProg) GetPinPath() string {
	return p.PinPath()
}

// BPFProgType is an enum as defined in https://elixir.bootlin.com/linux/latest/source/include/uapi/linux/bpf.h
type BPFProgType uint32

const (
	BPFProgTypeUnspec BPFProgType = iota
	BPFProgTypeSocketFilter
	BPFProgTypeKprobe
	BPFProgTypeSchedCls
	BPFProgTypeSchedAct
	BPFProgTypeTracepoint
	BPFProgTypeXdp
	BPFProgTypePerfEvent
	BPFProgTypeCgroupSkb
	BPFProgTypeCgroupSock
	BPFProgTypeLwtIn
	BPFProgTypeLwtOut
	BPFProgTypeLwtXmit
	BPFProgTypeSockOps
	BPFProgTypeSkSkb
	BPFProgTypeCgroupDevice
	BPFProgTypeSkMsg
	BPFProgTypeRawTracepoint
	BPFProgTypeCgroupSockAddr
	BPFProgTypeLwtSeg6Local
	BPFProgTypeLircMode2
	BPFProgTypeSkReuseport
	BPFProgTypeFlowDissector
	BPFProgTypeCgroupSysctl
	BPFProgTypeRawTracepointWritable
	BPFProgTypeCgroupSockopt
	BPFProgTypeTracing
	BPFProgTypeStructOps
	BPFProgTypeExt
	BPFProgTypeLsm
	BPFProgTypeSkLookup
	BPFProgTypeSyscall
)

func (b BPFProgType) Value() uint64 { return uint64(b) }

func (b BPFProgType) String() (str string) {
	x := map[BPFProgType]string{
		BPFProgTypeUnspec:                "BPF_PROG_TYPE_UNSPEC",
		BPFProgTypeSocketFilter:          "BPF_PROG_TYPE_SOCKET_FILTER",
		BPFProgTypeKprobe:                "BPF_PROG_TYPE_KPROBE",
		BPFProgTypeSchedCls:              "BPF_PROG_TYPE_SCHED_CLS",
		BPFProgTypeSchedAct:              "BPF_PROG_TYPE_SCHED_ACT",
		BPFProgTypeTracepoint:            "BPF_PROG_TYPE_TRACEPOINT",
		BPFProgTypeXdp:                   "BPF_PROG_TYPE_XDP",
		BPFProgTypePerfEvent:             "BPF_PROG_TYPE_PERF_EVENT",
		BPFProgTypeCgroupSkb:             "BPF_PROG_TYPE_CGROUP_SKB",
		BPFProgTypeCgroupSock:            "BPF_PROG_TYPE_CGROUP_SOCK",
		BPFProgTypeLwtIn:                 "BPF_PROG_TYPE_LWT_IN",
		BPFProgTypeLwtOut:                "BPF_PROG_TYPE_LWT_OUT",
		BPFProgTypeLwtXmit:               "BPF_PROG_TYPE_LWT_XMIT",
		BPFProgTypeSockOps:               "BPF_PROG_TYPE_SOCK_OPS",
		BPFProgTypeSkSkb:                 "BPF_PROG_TYPE_SK_SKB",
		BPFProgTypeCgroupDevice:          "BPF_PROG_TYPE_CGROUP_DEVICE",
		BPFProgTypeSkMsg:                 "BPF_PROG_TYPE_SK_MSG",
		BPFProgTypeRawTracepoint:         "BPF_PROG_TYPE_RAW_TRACEPOINT",
		BPFProgTypeCgroupSockAddr:        "BPF_PROG_TYPE_CGROUP_SOCK_ADDR",
		BPFProgTypeLwtSeg6Local:          "BPF_PROG_TYPE_LWT_SEG6LOCAL",
		BPFProgTypeLircMode2:             "BPF_PROG_TYPE_LIRC_MODE2",
		BPFProgTypeSkReuseport:           "BPF_PROG_TYPE_SK_REUSEPORT",
		BPFProgTypeFlowDissector:         "BPF_PROG_TYPE_FLOW_DISSECTOR",
		BPFProgTypeCgroupSysctl:          "BPF_PROG_TYPE_CGROUP_SYSCTL",
		BPFProgTypeRawTracepointWritable: "BPF_PROG_TYPE_RAW_TRACEPOINT_WRITABLE",
		BPFProgTypeCgroupSockopt:         "BPF_PROG_TYPE_CGROUP_SOCKOPT",
		BPFProgTypeTracing:               "BPF_PROG_TYPE_TRACING",
		BPFProgTypeStructOps:             "BPF_PROG_TYPE_STRUCT_OPS",
		BPFProgTypeExt:                   "BPF_PROG_TYPE_EXT",
		BPFProgTypeLsm:                   "BPF_PROG_TYPE_LSM",
		BPFProgTypeSkLookup:              "BPF_PROG_TYPE_SK_LOOKUP",
		BPFProgTypeSyscall:               "BPF_PROG_TYPE_SYSCALL",
	}
	str = x[b]
	if str == "" {
		str = BPFProgTypeUnspec.String()
	}
	return str
}

type BPFAttachType uint32

const (
	BPFAttachTypeCgroupInetIngress BPFAttachType = iota
	BPFAttachTypeCgroupInetEgress
	BPFAttachTypeCgroupInetSockCreate
	BPFAttachTypeCgroupSockOps
	BPFAttachTypeSKSKBStreamParser
	BPFAttachTypeSKSKBStreamVerdict
	BPFAttachTypeCgroupDevice
	BPFAttachTypeSKMSGVerdict
	BPFAttachTypeCgroupInet4Bind
	BPFAttachTypeCgroupInet6Bind
	BPFAttachTypeCgroupInet4Connect
	BPFAttachTypeCgroupInet6Connect
	BPFAttachTypeCgroupInet4PostBind
	BPFAttachTypeCgroupInet6PostBind
	BPFAttachTypeCgroupUDP4SendMsg
	BPFAttachTypeCgroupUDP6SendMsg
	BPFAttachTypeLircMode2
	BPFAttachTypeFlowDissector
	BPFAttachTypeCgroupSysctl
	BPFAttachTypeCgroupUDP4RecvMsg
	BPFAttachTypeCgroupUDP6RecvMsg
	BPFAttachTypeCgroupGetSockOpt
	BPFAttachTypeCgroupSetSockOpt
	BPFAttachTypeTraceRawTP
	BPFAttachTypeTraceFentry
	BPFAttachTypeTraceFexit
	BPFAttachTypeModifyReturn
	BPFAttachTypeLSMMac
	BPFAttachTypeTraceIter
	BPFAttachTypeCgroupInet4GetPeerName
	BPFAttachTypeCgroupInet6GetPeerName
	BPFAttachTypeCgroupInet4GetSockName
	BPFAttachTypeCgroupInet6GetSockName
	BPFAttachTypeXDPDevMap
	BPFAttachTypeCgroupInetSockRelease
	BPFAttachTypeXDPCPUMap
	BPFAttachTypeSKLookup
	BPFAttachTypeXDP
	BPFAttachTypeSKSKBVerdict
	BPFAttachTypeSKReusePortSelect
	BPFAttachTypeSKReusePortSelectorMigrate
	BPFAttachTypePerfEvent
	BPFAttachTypeTraceKprobeMulti
)

func (p *BPFProg) GetType() BPFProgType {
	return BPFProgType(C.bpf_program__type(p.prog))
}

func (p *BPFProg) SetAutoload(autoload bool) error {
	cbool := C.bool(autoload)
	ret := C.bpf_program__set_autoload(p.prog, cbool)
	if ret != 0 {
		return fmt.Errorf("failed to set bpf program autoload: %w", syscall.Errno(-ret))
	}
	return nil
}

// AttachGeneric is used to attach the BPF program using autodetection
// for the attach target. You can specify the destination in BPF code
// via the SEC() such as `SEC("fentry/some_kernel_func")`
func (p *BPFProg) AttachGeneric() (*BPFLink, error) {
	link, errno := C.bpf_program__attach(p.prog)
	if link == nil {
		return nil, fmt.Errorf("failed to attach program: %w", errno)
	}
	bpfLink := &BPFLink{
		link:      link,
		prog:      p,
		linkType:  Tracing,
		eventName: fmt.Sprintf("tracing-%s", p.name),
	}
	return bpfLink, nil
}

// SetAttachTarget can be used to specify the program and/or function to attach
// the BPF program to. To attach to a kernel function specify attachProgFD as 0
func (p *BPFProg) SetAttachTarget(attachProgFD int, attachFuncName string) error {
	cs := C.CString(attachFuncName)
	ret := C.bpf_program__set_attach_target(p.prog, C.int(attachProgFD), cs)
	C.free(unsafe.Pointer(cs))
	if ret != 0 {
		return fmt.Errorf("failed to set attach target for program %s %s %w", p.name, attachFuncName, syscall.Errno(-ret))
	}
	return nil
}

func (p *BPFProg) SetProgramType(progType BPFProgType) {
	C.bpf_program__set_type(p.prog, C.enum_bpf_prog_type(int(progType)))
}

func (p *BPFProg) SetAttachType(attachType BPFAttachType) {
	C.bpf_program__set_expected_attach_type(p.prog, C.enum_bpf_attach_type(int(attachType)))
}

// getCgroupDirFD returns a file descriptor for a given cgroup2 directory path
func getCgroupDirFD(cgroupV2DirPath string) (int, error) {
	// revive:disable
	const (
		O_DIRECTORY int = syscall.O_DIRECTORY
		O_RDONLY    int = syscall.O_RDONLY
	)
	// revive:enable
	fd, err := syscall.Open(cgroupV2DirPath, O_DIRECTORY|O_RDONLY, 0)
	if fd < 0 {
		return 0, fmt.Errorf("failed to open cgroupv2 directory path %s: %w", cgroupV2DirPath, err)
	}
	return fd, nil
}

// AttachCgroup attaches the BPFProg to a cgroup described by given fd.
func (p *BPFProg) AttachCgroup(cgroupV2DirPath string) (*BPFLink, error) {
	cgroupDirFD, err := getCgroupDirFD(cgroupV2DirPath)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(cgroupDirFD)

	link, errno := C.bpf_program__attach_cgroup(p.prog, C.int(cgroupDirFD))
	if link == nil {
		return nil, fmt.Errorf("failed to attach cgroup on cgroupv2 %s to program %s: %w", cgroupV2DirPath, p.name, errno)
	}

	// dirName will be used in bpfLink.eventName. eventName follows a format
	// convention and is used to better identify link types and what they are
	// linked with in case of errors or similar needs. Having eventName as:
	// cgroup-progName-/sys/fs/cgroup/unified/ would look weird so replace it
	// to be cgroup-progName-sys-fs-cgroup-unified instead.
	dirName := strings.ReplaceAll(cgroupV2DirPath[1:], "/", "-")
	bpfLink := &BPFLink{
		link:      link,
		prog:      p,
		linkType:  Cgroup,
		eventName: fmt.Sprintf("cgroup-%s-%s", p.name, dirName),
	}
	p.module.links = append(p.module.links, bpfLink)
	return bpfLink, nil
}

// AttachCgroupLegacy attaches the BPFProg to a cgroup described by the given
// fd. It first tries to use the most recent attachment method and, if that does
// not work, instead of failing, it tries the legacy way: to attach the cgroup
// eBPF program without previously creating a link. This allows attaching cgroup
// eBPF ingress/egress in older kernels. Note: the first attempt error message
// is filtered out inside libbpf_print_fn() as it is actually a feature probe
// attempt as well.
//
// Related kernel commit: https://github.com/torvalds/linux/commit/af6eea57437a
func (p *BPFProg) AttachCgroupLegacy(cgroupV2DirPath string, attachType BPFAttachType) (*BPFLink, error) {
	bpfLink, err := p.AttachCgroup(cgroupV2DirPath)
	if err == nil {
		return bpfLink, nil
	}
	// Try the legacy attachment method before fully failing
	cgroupDirFD, err := getCgroupDirFD(cgroupV2DirPath)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(cgroupDirFD)
	progFD := C.bpf_program__fd(p.prog)
	ret := C.cgo_bpf_prog_attach_cgroup_legacy(progFD, C.int(cgroupDirFD), C.int(attachType))
	if ret < 0 {
		return nil, fmt.Errorf("failed to attach (legacy) program %s to cgroupv2 %s", p.name, cgroupV2DirPath)
	}
	dirName := strings.ReplaceAll(cgroupV2DirPath[1:], "/", "-")

	bpfLinkLegacy := &BPFLinkLegacy{
		attachType: attachType,
		cgroupDir:  cgroupV2DirPath,
	}
	fakeBpfLink := &BPFLink{
		link:      nil, // detach/destroy made with progfd
		prog:      p,
		eventName: fmt.Sprintf("cgroup-%s-%s", p.name, dirName),
		// info bellow needed for detach (there isn't a real ebpf link)
		linkType: CgroupLegacy,
		legacy:   bpfLinkLegacy,
	}
	return fakeBpfLink, nil
}

// DetachCgroupLegacy detaches the BPFProg from a cgroup described by the given
// fd. This is needed because in legacy attachment there is no BPFLink, just a
// fake one (kernel did not support it, nor libbpf). This function should be
// called by the (*BPFLink)->Destroy() function, since BPFLink is emulated (so
// users don´t need to distinguish between regular and legacy cgroup
// detachments).
func (p *BPFProg) DetachCgroupLegacy(cgroupV2DirPath string, attachType BPFAttachType) error {
	cgroupDirFD, err := getCgroupDirFD(cgroupV2DirPath)
	if err != nil {
		return err
	}
	defer syscall.Close(cgroupDirFD)
	progFD := C.bpf_program__fd(p.prog)
	ret := C.cgo_bpf_prog_detach_cgroup_legacy(progFD, C.int(cgroupDirFD), C.int(attachType))
	if ret < 0 {
		return fmt.Errorf("failed to detach (legacy) program %s from cgroupv2 %s", p.name, cgroupV2DirPath)
	}
	return nil
}

func (p *BPFProg) AttachXDP(deviceName string) (*BPFLink, error) {
	iface, err := net.InterfaceByName(deviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to find device by name %s: %w", deviceName, err)
	}
	link, errno := C.bpf_program__attach_xdp(p.prog, C.int(iface.Index))
	if link == nil {
		return nil, fmt.Errorf("failed to attach xdp on device %s to program %s: %w", deviceName, p.name, errno)
	}

	bpfLink := &BPFLink{
		link:      link,
		prog:      p,
		linkType:  XDP,
		eventName: fmt.Sprintf("xdp-%s-%s", p.name, deviceName),
	}
	p.module.links = append(p.module.links, bpfLink)
	return bpfLink, nil
}

func (p *BPFProg) AttachTracepoint(category, name string) (*BPFLink, error) {
	tpCategory := C.CString(category)
	tpName := C.CString(name)
	link, errno := C.bpf_program__attach_tracepoint(p.prog, tpCategory, tpName)
	C.free(unsafe.Pointer(tpCategory))
	C.free(unsafe.Pointer(tpName))
	if link == nil {
		return nil, fmt.Errorf("failed to attach tracepoint %s to program %s: %w", name, p.name, errno)
	}

	bpfLink := &BPFLink{
		link:      link,
		prog:      p,
		linkType:  Tracepoint,
		eventName: name,
	}
	p.module.links = append(p.module.links, bpfLink)
	return bpfLink, nil
}

func (p *BPFProg) AttachRawTracepoint(tpEvent string) (*BPFLink, error) {
	cs := C.CString(tpEvent)
	link, errno := C.bpf_program__attach_raw_tracepoint(p.prog, cs)
	C.free(unsafe.Pointer(cs))
	if link == nil {
		return nil, fmt.Errorf("failed to attach raw tracepoint %s to program %s: %w", tpEvent, p.name, errno)
	}

	bpfLink := &BPFLink{
		link:      link,
		prog:      p,
		linkType:  RawTracepoint,
		eventName: tpEvent,
	}
	p.module.links = append(p.module.links, bpfLink)
	return bpfLink, nil
}

func (p *BPFProg) AttachLSM() (*BPFLink, error) {
	link, errno := C.bpf_program__attach_lsm(p.prog)
	if link == nil {
		return nil, fmt.Errorf("failed to attach lsm to program %s: %w", p.name, errno)
	}

	bpfLink := &BPFLink{
		link:     link,
		prog:     p,
		linkType: LSM,
	}
	p.module.links = append(p.module.links, bpfLink)
	return bpfLink, nil
}

func (p *BPFProg) AttachPerfEvent(fd int) (*BPFLink, error) {
	link, errno := C.bpf_program__attach_perf_event(p.prog, C.int(fd))
	if link == nil {
		return nil, fmt.Errorf("failed to attach perf event to program %s: %w", p.name, errno)
	}

	bpfLink := &BPFLink{
		link:     link,
		prog:     p,
		linkType: PerfEvent,
	}
	p.module.links = append(p.module.links, bpfLink)
	return bpfLink, nil
}

// this API should be used for kernels > 4.17
func (p *BPFProg) AttachKprobe(kp string) (*BPFLink, error) {
	return doAttachKprobe(p, kp, false)
}

// this API should be used for kernels > 4.17
func (p *BPFProg) AttachKretprobe(kp string) (*BPFLink, error) {
	return doAttachKprobe(p, kp, true)
}

func (p *BPFProg) AttachNetns(networkNamespacePath string) (*BPFLink, error) {
	fd, err := syscall.Open(networkNamespacePath, syscall.O_RDONLY, 0)
	if fd < 0 {
		return nil, fmt.Errorf("failed to open network namespace path %s: %w", networkNamespacePath, err)
	}
	link, errno := C.bpf_program__attach_netns(p.prog, C.int(fd))
	if link == nil {
		return nil, fmt.Errorf("failed to attach network namespace on %s to program %s: %w", networkNamespacePath, p.name, errno)
	}

	// fileName will be used in bpfLink.eventName. eventName follows a format
	// convention and is used to better identify link types and what they are
	// linked with in case of errors or similar needs. Having eventName as:
	// netns-progName-/proc/self/ns/net would look weird so replace it
	// to be netns-progName-proc-self-ns-net instead.
	fileName := strings.ReplaceAll(networkNamespacePath[1:], "/", "-")
	bpfLink := &BPFLink{
		link:      link,
		prog:      p,
		linkType:  Netns,
		eventName: fmt.Sprintf("netns-%s-%s", p.name, fileName),
	}
	p.module.links = append(p.module.links, bpfLink)
	return bpfLink, nil
}

type BPFCgroupIterOrder uint32

const (
	BPFIterOrderUnspec BPFCgroupIterOrder = iota
	BPFIterSelfOnly
	BPFIterDescendantsPre
	BPFIterDescendantsPost
	BPFIterAncestorsUp
)

type IterOpts struct {
	MapFd           int
	CgroupIterOrder BPFCgroupIterOrder
	CgroupFd        int
	CgroupId        uint64
	Tid             int
	Pid             int
	PidFd           int
}

func (p *BPFProg) AttachIter(opts IterOpts) (*BPFLink, error) {
	mapFd := C.uint(opts.MapFd)
	cgroupIterOrder := uint32(opts.CgroupIterOrder)
	cgroupFd := C.uint(opts.CgroupFd)
	cgroupId := C.ulonglong(opts.CgroupId)
	tid := C.uint(opts.Tid)
	pid := C.uint(opts.Pid)
	pidFd := C.uint(opts.PidFd)
	cOpts, errno := C.cgo_bpf_iter_attach_opts_new(mapFd, cgroupIterOrder, cgroupFd, cgroupId, tid, pid, pidFd)
	if cOpts == nil {
		return nil, fmt.Errorf("failed to create iter_attach_opts to program %s: %w", p.name, errno)
	}
	defer C.cgo_bpf_iter_attach_opts_free(cOpts)

	link, errno := C.bpf_program__attach_iter(p.prog, cOpts)
	if link == nil {
		return nil, fmt.Errorf("failed to attach iter to program %s: %w", p.name, errno)
	}
	eventName := fmt.Sprintf("iter-%s-%d", p.name, opts.MapFd)
	bpfLink := &BPFLink{
		link:      link,
		prog:      p,
		linkType:  Iter,
		eventName: eventName,
	}
	p.module.links = append(p.module.links, bpfLink)
	return bpfLink, nil
}

func doAttachKprobe(prog *BPFProg, kp string, isKretprobe bool) (*BPFLink, error) {
	cs := C.CString(kp)
	cbool := C.bool(isKretprobe)
	link, errno := C.bpf_program__attach_kprobe(prog.prog, cbool, cs)
	C.free(unsafe.Pointer(cs))
	if link == nil {
		return nil, fmt.Errorf("failed to attach %s k(ret)probe to program %s: %w", kp, prog.name, errno)
	}

	kpType := Kprobe
	if isKretprobe {
		kpType = Kretprobe
	}

	bpfLink := &BPFLink{
		link:      link,
		prog:      prog,
		linkType:  kpType,
		eventName: kp,
	}
	prog.module.links = append(prog.module.links, bpfLink)
	return bpfLink, nil
}

// AttachUprobe attaches the BPFProgram to entry of the symbol in the library or binary at 'path'
// which can be relative or absolute. A pid can be provided to attach to, or -1 can be specified
// to attach to all processes
func (p *BPFProg) AttachUprobe(pid int, path string, offset uint32) (*BPFLink, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	return doAttachUprobe(p, false, pid, absPath, offset)
}

// AttachURetprobe attaches the BPFProgram to exit of the symbol in the library or binary at 'path'
// which can be relative or absolute. A pid can be provided to attach to, or -1 can be specified
// to attach to all processes
func (p *BPFProg) AttachURetprobe(pid int, path string, offset uint32) (*BPFLink, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	return doAttachUprobe(p, true, pid, absPath, offset)
}

func doAttachUprobe(prog *BPFProg, isUretprobe bool, pid int, path string, offset uint32) (*BPFLink, error) {
	retCBool := C.bool(isUretprobe)
	pidCint := C.int(pid)
	pathCString := C.CString(path)
	offsetCsizet := C.size_t(offset)

	link, errno := C.bpf_program__attach_uprobe(prog.prog, retCBool, pidCint, pathCString, offsetCsizet)
	C.free(unsafe.Pointer(pathCString))
	if link == nil {
		return nil, fmt.Errorf("failed to attach u(ret)probe to program %s:%d with pid %d: %w ", path, offset, pid, errno)
	}

	upType := Uprobe
	if isUretprobe {
		upType = Uretprobe
	}

	bpfLink := &BPFLink{
		link:      link,
		prog:      prog,
		linkType:  upType,
		eventName: fmt.Sprintf("%s:%d:%d", path, pid, offset),
	}
	return bpfLink, nil
}

type AttachFlag uint32

const (
	BPFFNone          AttachFlag = 0
	BPFFAllowOverride AttachFlag = C.BPF_F_ALLOW_OVERRIDE
	BPFFAllowMulti    AttachFlag = C.BPF_F_ALLOW_MULTI
	BPFFReplace       AttachFlag = C.BPF_F_REPLACE
)

// AttachGenericFD attaches the BPFProgram to a targetFd at the specified attachType hook.
func (p *BPFProg) AttachGenericFD(targetFd int, attachType BPFAttachType, flags AttachFlag) error {
	progFd := C.bpf_program__fd(p.prog)
	errC := C.bpf_prog_attach(progFd, C.int(targetFd), C.enum_bpf_attach_type(int(attachType)), C.uint(uint(flags)))
	if errC < 0 {
		return fmt.Errorf("failed to attach: %w", syscall.Errno(-errC))
	}
	return nil
}

// DetachGenericFD detaches the BPFProgram associated with the targetFd at the hook specified by attachType.
func (p *BPFProg) DetachGenericFD(targetFd int, attachType BPFAttachType) error {
	progFd := C.bpf_program__fd(p.prog)
	errC := C.bpf_prog_detach2(progFd, C.int(targetFd), C.enum_bpf_attach_type(int(attachType)))
	if errC < 0 {
		return fmt.Errorf("failed to detach: %w", syscall.Errno(-errC))
	}
	return nil
}

var eventChannels = newRWArray(maxEventChannels)

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

	rb := C.cgo_init_ring_buf(C.int(bpfMap.FileDescriptor()), C.uintptr_t(slot))
	if rb == nil {
		return nil, fmt.Errorf("failed to initialize ring buffer")
	}

	ringBuf := &RingBuffer{
		rb:     rb,
		bpfMap: bpfMap,
		slot:   uint(slot),
	}
	m.ringBufs = append(m.ringBufs, ringBuf)
	return ringBuf, nil
}

// Poll will wait until timeout in milliseconds to gather
// data from the ring buffer.
func (rb *RingBuffer) Poll(timeout int) {
	rb.stop = make(chan struct{})
	rb.wg.Add(1)
	go rb.poll(timeout)
}

// Deprecated: use RingBuffer.Poll() instead.
func (rb *RingBuffer) Start() {
	rb.Poll(300)
}

func (rb *RingBuffer) Stop() {
	if rb.stop != nil {
		// Tell the poll goroutine that it's time to exit
		close(rb.stop)

		// The event channel should be drained here since the consumer
		// may have stopped at this point. Failure to drain it will
		// result in a deadlock: the channel will fill up and the poll
		// goroutine will block in the callback.
		eventChan := eventChannels.get(rb.slot).(chan []byte)
		go func() {
			// revive:disable:empty-block
			for range eventChan {
			}
			// revive:enable:empty-block
		}()

		// Wait for the poll goroutine to exit
		rb.wg.Wait()

		// Close the channel -- this is useful for the consumer but
		// also to terminate the drain goroutine above.
		close(eventChan)

		// This allows Stop() to be called multiple times safely
		rb.stop = nil
	}
}

func (rb *RingBuffer) Close() {
	if rb.closed {
		return
	}
	rb.Stop()
	C.ring_buffer__free(rb.rb)
	eventChannels.remove(rb.slot)
	rb.closed = true
}

func (rb *RingBuffer) isStopped() bool {
	select {
	case <-rb.stop:
		return true
	default:
		return false
	}
}

func (rb *RingBuffer) poll(timeout int) error {
	defer rb.wg.Done()

	for {
		err := C.ring_buffer__poll(rb.rb, C.int(timeout))
		if rb.isStopped() {
			break
		}

		if err < 0 {
			if syscall.Errno(-err) == syscall.EINTR {
				continue
			}
			return fmt.Errorf("error polling ring buffer: %d", err)
		}
	}
	return nil
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

	pb := C.cgo_init_perf_buf(C.int(bpfMap.FileDescriptor()), C.int(pageCnt), C.uintptr_t(slot))
	if pb == nil {
		eventChannels.remove(uint(slot))
		return nil, fmt.Errorf("failed to initialize perf buffer")
	}

	perfBuf.pb = pb
	perfBuf.slot = uint(slot)

	m.perfBufs = append(m.perfBufs, perfBuf)
	return perfBuf, nil
}

// Poll will wait until timeout in milliseconds to gather
// data from the perf buffer.
func (pb *PerfBuffer) Poll(timeout int) {
	pb.stop = make(chan struct{})
	pb.wg.Add(1)
	go pb.poll(timeout)
}

// Deprecated: use PerfBuffer.Poll() instead.
func (pb *PerfBuffer) Start() {
	pb.Poll(300)
}

func (pb *PerfBuffer) Stop() {
	if pb.stop != nil {
		// Tell the poll goroutine that it's time to exit
		close(pb.stop)

		// The event and lost channels should be drained here since the consumer
		// may have stopped at this point. Failure to drain it will
		// result in a deadlock: the channel will fill up and the poll
		// goroutine will block in the callback.
		go func() {
			// revive:disable:empty-block
			for range pb.eventsChan {
			}

			if pb.lostChan != nil {
				for range pb.lostChan {
				}
			}
			// revive:enable:empty-block
		}()

		// Wait for the poll goroutine to exit
		pb.wg.Wait()

		// Close the channel -- this is useful for the consumer but
		// also to terminate the drain goroutine above.
		close(pb.eventsChan)
		if pb.lostChan != nil {
			close(pb.lostChan)
		}

		// This allows Stop() to be called multiple times safely
		pb.stop = nil
	}
}

func (pb *PerfBuffer) Close() {
	if pb.closed {
		return
	}
	pb.Stop()
	C.perf_buffer__free(pb.pb)
	eventChannels.remove(pb.slot)
	pb.closed = true
}

// todo: consider writing the perf polling in go as c to go calls (callback) are expensive
func (pb *PerfBuffer) poll(timeout int) error {
	defer pb.wg.Done()

	for {
		select {
		case <-pb.stop:
			return nil
		default:
			err := C.perf_buffer__poll(pb.pb, C.int(timeout))
			if err < 0 {
				if syscall.Errno(-err) == syscall.EINTR {
					continue
				}
				return fmt.Errorf("error polling perf buffer: %d", err)
			}
		}
	}
}

type TcAttachPoint uint32

const (
	BPFTcIngress       TcAttachPoint = C.BPF_TC_INGRESS
	BPFTcEgress        TcAttachPoint = C.BPF_TC_EGRESS
	BPFTcIngressEgress TcAttachPoint = C.BPF_TC_INGRESS | C.BPF_TC_EGRESS
	BPFTcCustom        TcAttachPoint = C.BPF_TC_CUSTOM
)

type TcFlags uint32

const (
	BpfTcFReplace TcFlags = C.BPF_TC_F_REPLACE
)

type TcHook struct {
	hook *C.struct_bpf_tc_hook
}

type TcOpts struct {
	ProgFd   int
	Flags    TcFlags
	ProgId   uint
	Handle   uint
	Priority uint
}

func tcOptsToC(tcOpts *TcOpts) *C.struct_bpf_tc_opts {
	if tcOpts == nil {
		return nil
	}
	opts := C.struct_bpf_tc_opts{}
	opts.sz = C.sizeof_struct_bpf_tc_opts
	opts.prog_fd = C.int(tcOpts.ProgFd)
	opts.flags = C.uint(tcOpts.Flags)
	opts.prog_id = C.uint(tcOpts.ProgId)
	opts.handle = C.uint(tcOpts.Handle)
	opts.priority = C.uint(tcOpts.Priority)

	return &opts
}

func tcOptsFromC(tcOpts *TcOpts, opts *C.struct_bpf_tc_opts) {
	if opts == nil {
		return
	}
	tcOpts.ProgFd = int(opts.prog_fd)
	tcOpts.Flags = TcFlags(opts.flags)
	tcOpts.ProgId = uint(opts.prog_id)
	tcOpts.Handle = uint(opts.handle)
	tcOpts.Priority = uint(opts.priority)
}

func (m *Module) TcHookInit() *TcHook {
	hook := C.struct_bpf_tc_hook{}
	hook.sz = C.sizeof_struct_bpf_tc_hook

	return &TcHook{
		hook: &hook,
	}
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
	errC := C.bpf_tc_hook_create(hook.hook)
	if errC < 0 {
		return fmt.Errorf("failed to create tc hook: %w", syscall.Errno(-errC))
	}

	return nil
}

func (hook *TcHook) Destroy() error {
	errC := C.bpf_tc_hook_destroy(hook.hook)
	if errC < 0 {
		return fmt.Errorf("failed to destroy tc hook: %w", syscall.Errno(-errC))
	}

	return nil
}

func (hook *TcHook) Attach(tcOpts *TcOpts) error {
	opts := tcOptsToC(tcOpts)
	errC := C.bpf_tc_attach(hook.hook, opts)
	if errC < 0 {
		return fmt.Errorf("failed to attach tc hook: %w", syscall.Errno(-errC))
	}
	tcOptsFromC(tcOpts, opts)

	return nil
}

func (hook *TcHook) Detach(tcOpts *TcOpts) error {
	opts := tcOptsToC(tcOpts)
	errC := C.bpf_tc_detach(hook.hook, opts)
	if errC < 0 {
		return fmt.Errorf("failed to detach tc hook: %w", syscall.Errno(-errC))
	}
	tcOptsFromC(tcOpts, opts)

	return nil
}

func (hook *TcHook) Query(tcOpts *TcOpts) error {
	opts := tcOptsToC(tcOpts)
	errC := C.bpf_tc_query(hook.hook, opts)
	if errC < 0 {
		return fmt.Errorf("failed to query tc hook: %w", syscall.Errno(-errC))
	}
	tcOptsFromC(tcOpts, opts)

	return nil
}

func BPFMapTypeIsSupported(mapType MapType) (bool, error) {
	cSupported := C.libbpf_probe_bpf_map_type(C.enum_bpf_map_type(int(mapType)), nil)
	if cSupported < 1 {
		return false, syscall.Errno(-cSupported)
	}
	return cSupported == 1, nil
}

func BPFProgramTypeIsSupported(progType BPFProgType) (bool, error) {
	cSupported := C.libbpf_probe_bpf_prog_type(C.enum_bpf_prog_type(int(progType)), nil)
	if cSupported < 1 {
		return false, syscall.Errno(-cSupported)
	}
	return cSupported == 1, nil
}

func NumPossibleCPUs() (int, error) {
	numCPUs, errC := C.libbpf_num_possible_cpus()
	if numCPUs < 0 {
		return 0, fmt.Errorf("failed to retrieve the number of CPUs: %w", errC)
	}
	return int(numCPUs), nil
}
