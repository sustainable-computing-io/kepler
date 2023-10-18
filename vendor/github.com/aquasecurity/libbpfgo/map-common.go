package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

import (
	"fmt"
	"syscall"
)

//
// MapType
//

type MapType uint32

const (
	MapTypeUnspec MapType = iota
	MapTypeHash
	MapTypeArray
	MapTypeProgArray
	MapTypePerfEventArray
	MapTypePerCPUHash
	MapTypePerCPUArray
	MapTypeStackTrace
	MapTypeCgroupArray
	MapTypeLRUHash
	MapTypeLRUPerCPUHash
	MapTypeLPMTrie
	MapTypeArrayOfMaps
	MapTypeHashOfMaps
	MapTypeDevMap
	MapTypeSockMap
	MapTypeCPUMap
	MapTypeXSKMap
	MapTypeSockHash
	MapTypeCgroupStorage
	MapTypeReusePortSockArray
	MapTypePerCPUCgroupStorage
	MapTypeQueue
	MapTypeStack
	MapTypeSKStorage
	MapTypeDevmapHash
	MapTypeStructOps
	MapTypeRingbuf
	MapTypeInodeStorage
	MapTypeTaskStorage
	MapTypeBloomFilter
)

var mapTypeToString = map[MapType]string{
	MapTypeUnspec:              "BPF_MAP_TYPE_UNSPEC",
	MapTypeHash:                "BPF_MAP_TYPE_HASH",
	MapTypeArray:               "BPF_MAP_TYPE_ARRAY",
	MapTypeProgArray:           "BPF_MAP_TYPE_PROG_ARRAY",
	MapTypePerfEventArray:      "BPF_MAP_TYPE_PERF_EVENT_ARRAY",
	MapTypePerCPUHash:          "BPF_MAP_TYPE_PERCPU_HASH",
	MapTypePerCPUArray:         "BPF_MAP_TYPE_PERCPU_ARRAY",
	MapTypeStackTrace:          "BPF_MAP_TYPE_STACK_TRACE",
	MapTypeCgroupArray:         "BPF_MAP_TYPE_CGROUP_ARRAY",
	MapTypeLRUHash:             "BPF_MAP_TYPE_LRU_HASH",
	MapTypeLRUPerCPUHash:       "BPF_MAP_TYPE_LRU_PERCPU_HASH",
	MapTypeLPMTrie:             "BPF_MAP_TYPE_LPM_TRIE",
	MapTypeArrayOfMaps:         "BPF_MAP_TYPE_ARRAY_OF_MAPS",
	MapTypeHashOfMaps:          "BPF_MAP_TYPE_HASH_OF_MAPS",
	MapTypeDevMap:              "BPF_MAP_TYPE_DEVMAP",
	MapTypeSockMap:             "BPF_MAP_TYPE_SOCKMAP",
	MapTypeCPUMap:              "BPF_MAP_TYPE_CPUMAP",
	MapTypeXSKMap:              "BPF_MAP_TYPE_XSKMAP",
	MapTypeSockHash:            "BPF_MAP_TYPE_SOCKHASH",
	MapTypeCgroupStorage:       "BPF_MAP_TYPE_CGROUP_STORAGE",
	MapTypeReusePortSockArray:  "BPF_MAP_TYPE_REUSEPORT_SOCKARRAY",
	MapTypePerCPUCgroupStorage: "BPF_MAP_TYPE_PERCPU_CGROUP_STORAGE",
	MapTypeQueue:               "BPF_MAP_TYPE_QUEUE",
	MapTypeStack:               "BPF_MAP_TYPE_STACK",
	MapTypeSKStorage:           "BPF_MAP_TYPE_SK_STORAGE",
	MapTypeDevmapHash:          "BPF_MAP_TYPE_DEVMAP_HASH",
	MapTypeStructOps:           "BPF_MAP_TYPE_STRUCT_OPS",
	MapTypeRingbuf:             "BPF_MAP_TYPE_RINGBUF",
	MapTypeInodeStorage:        "BPF_MAP_TYPE_INODE_STORAGE",
	MapTypeTaskStorage:         "BPF_MAP_TYPE_TASK_STORAGE",
	MapTypeBloomFilter:         "BPF_MAP_TYPE_BLOOM_FILTER",
}

func (t MapType) String() string {
	return mapTypeToString[t]
}

//
// MapFlag
//

type MapFlag uint32

const (
	MapFlagUpdateAny     MapFlag = iota // create new element or update existing
	MapFlagUpdateNoExist                // create new element if it didn't exist
	MapFlagUpdateExist                  // update existing element
	MapFlagFLock                        // spin_lock-ed map_lookup/map_update
)

//
// BPFMapInfo
//

// BPFMapInfo mirrors the C structure bpf_map_info.
type BPFMapInfo struct {
	Type                  MapType
	ID                    uint32
	KeySize               uint32
	ValueSize             uint32
	MaxEntries            uint32
	MapFlags              uint32
	Name                  string
	IfIndex               uint32
	BTFVmlinuxValueTypeID uint32
	NetnsDev              uint64
	NetnsIno              uint64
	BTFID                 uint32
	BTFKeyTypeID          uint32
	BTFValueTypeID        uint32
	MapExtra              uint64
}

// GetMapFDByID returns a file descriptor for the map with the given ID.
func GetMapFDByID(id uint32) (int, error) {
	fdC := C.bpf_map_get_fd_by_id(C.uint(id))
	if fdC < 0 {
		return int(fdC), fmt.Errorf("could not find map id %d: %w", id, syscall.Errno(-fdC))
	}

	return int(fdC), nil
}

// GetMapInfoByFD returns the BPFMapInfo for the map with the given file descriptor.
func GetMapInfoByFD(fd int) (*BPFMapInfo, error) {
	var info C.struct_bpf_map_info
	var infoLen C.uint = C.uint(C.sizeof_struct_bpf_map_info)

	retC := C.bpf_map_get_info_by_fd(C.int(fd), &info, &infoLen)
	if retC < 0 {
		return nil, fmt.Errorf("failed to get map info for fd %d: %w", fd, syscall.Errno(-retC))
	}

	return &BPFMapInfo{
		Type:                  MapType(uint32(info._type)),
		ID:                    uint32(info.id),
		KeySize:               uint32(info.key_size),
		ValueSize:             uint32(info.value_size),
		MaxEntries:            uint32(info.max_entries),
		MapFlags:              uint32(info.map_flags),
		Name:                  C.GoString(&info.name[0]),
		IfIndex:               uint32(info.ifindex),
		BTFVmlinuxValueTypeID: uint32(info.btf_vmlinux_value_type_id),
		NetnsDev:              uint64(info.netns_dev),
		NetnsIno:              uint64(info.netns_ino),
		BTFID:                 uint32(info.btf_id),
		BTFKeyTypeID:          uint32(info.btf_key_type_id),
		BTFValueTypeID:        uint32(info.btf_value_type_id),
		MapExtra:              uint64(info.map_extra),
	}, nil
}

//
// Map misc internal
//

// calcMapValueSize calculates the size of the value for a map.
// For per-CPU maps, it is calculated based on the number of possible CPUs.
func calcMapValueSize(valueSize int, mapType MapType) (int, error) {
	if valueSize <= 0 {
		return 0, fmt.Errorf("value size must be greater than 0")
	}

	switch mapType {
	case MapTypePerCPUArray,
		MapTypePerCPUHash,
		MapTypeLRUPerCPUHash,
		MapTypePerCPUCgroupStorage:
		// per-CPU maps have a value size calculated using a round-up of the
		// element size multiplied by the number of possible CPUs.
		elemSize := roundUp(uint64(valueSize), 8)
		numCPU, err := NumPossibleCPUs()
		if err != nil {
			return 0, err
		}

		return int(elemSize) * numCPU, nil
	default:
		// For other maps, the value size does not change.
		return valueSize, nil
	}
}
