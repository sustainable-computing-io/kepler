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
	MapTypeUnspec              MapType = C.BPF_MAP_TYPE_UNSPEC
	MapTypeHash                MapType = C.BPF_MAP_TYPE_HASH
	MapTypeArray               MapType = C.BPF_MAP_TYPE_ARRAY
	MapTypeProgArray           MapType = C.BPF_MAP_TYPE_PROG_ARRAY
	MapTypePerfEventArray      MapType = C.BPF_MAP_TYPE_PERF_EVENT_ARRAY
	MapTypePerCPUHash          MapType = C.BPF_MAP_TYPE_PERCPU_HASH
	MapTypePerCPUArray         MapType = C.BPF_MAP_TYPE_PERCPU_ARRAY
	MapTypeStackTrace          MapType = C.BPF_MAP_TYPE_STACK_TRACE
	MapTypeCgroupArray         MapType = C.BPF_MAP_TYPE_CGROUP_ARRAY
	MapTypeLRUHash             MapType = C.BPF_MAP_TYPE_LRU_HASH
	MapTypeLRUPerCPUHash       MapType = C.BPF_MAP_TYPE_LRU_PERCPU_HASH
	MapTypeLPMTrie             MapType = C.BPF_MAP_TYPE_LPM_TRIE
	MapTypeArrayOfMaps         MapType = C.BPF_MAP_TYPE_ARRAY_OF_MAPS
	MapTypeHashOfMaps          MapType = C.BPF_MAP_TYPE_HASH_OF_MAPS
	MapTypeDevMap              MapType = C.BPF_MAP_TYPE_DEVMAP
	MapTypeSockMap             MapType = C.BPF_MAP_TYPE_SOCKMAP
	MapTypeCPUMap              MapType = C.BPF_MAP_TYPE_CPUMAP
	MapTypeXSKMap              MapType = C.BPF_MAP_TYPE_XSKMAP
	MapTypeSockHash            MapType = C.BPF_MAP_TYPE_SOCKHASH
	MapTypeCgroupStorage       MapType = C.BPF_MAP_TYPE_CGROUP_STORAGE
	MapTypeReusePortSockArray  MapType = C.BPF_MAP_TYPE_REUSEPORT_SOCKARRAY
	MapTypePerCPUCgroupStorage MapType = C.BPF_MAP_TYPE_PERCPU_CGROUP_STORAGE
	MapTypeQueue               MapType = C.BPF_MAP_TYPE_QUEUE
	MapTypeStack               MapType = C.BPF_MAP_TYPE_STACK
	MapTypeSKStorage           MapType = C.BPF_MAP_TYPE_SK_STORAGE
	MapTypeDevmapHash          MapType = C.BPF_MAP_TYPE_DEVMAP_HASH
	MapTypeStructOps           MapType = C.BPF_MAP_TYPE_STRUCT_OPS
	MapTypeRingbuf             MapType = C.BPF_MAP_TYPE_RINGBUF
	MapTypeInodeStorage        MapType = C.BPF_MAP_TYPE_INODE_STORAGE
	MapTypeTaskStorage         MapType = C.BPF_MAP_TYPE_TASK_STORAGE
	MapTypeBloomFilter         MapType = C.BPF_MAP_TYPE_BLOOM_FILTER
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
	str, ok := mapTypeToString[t]
	if !ok {
		// MapTypeUnspec must exist in mapTypeToString to avoid infinite recursion.
		return BPFProgTypeUnspec.String()
	}

	return str
}

func (t MapType) Name() string {
	return C.GoString(C.libbpf_bpf_map_type_str(C.enum_bpf_map_type(t)))
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
	infoC := C.cgo_bpf_map_info_new()
	defer C.cgo_bpf_map_info_free(infoC)

	infoLenC := C.cgo_bpf_map_info_size()
	retC := C.bpf_map_get_info_by_fd(C.int(fd), infoC, &infoLenC)
	if retC < 0 {
		return nil, fmt.Errorf("failed to get map info for fd %d: %w", fd, syscall.Errno(-retC))
	}

	return &BPFMapInfo{
		Type:                  MapType(C.cgo_bpf_map_info_type(infoC)),
		ID:                    uint32(C.cgo_bpf_map_info_id(infoC)),
		KeySize:               uint32(C.cgo_bpf_map_info_key_size(infoC)),
		ValueSize:             uint32(C.cgo_bpf_map_info_value_size(infoC)),
		MaxEntries:            uint32(C.cgo_bpf_map_info_max_entries(infoC)),
		MapFlags:              uint32(C.cgo_bpf_map_info_map_flags(infoC)),
		Name:                  C.GoString(C.cgo_bpf_map_info_name(infoC)),
		IfIndex:               uint32(C.cgo_bpf_map_info_ifindex(infoC)),
		BTFVmlinuxValueTypeID: uint32(C.cgo_bpf_map_info_btf_vmlinux_value_type_id(infoC)),
		NetnsDev:              uint64(C.cgo_bpf_map_info_netns_dev(infoC)),
		NetnsIno:              uint64(C.cgo_bpf_map_info_netns_ino(infoC)),
		BTFID:                 uint32(C.cgo_bpf_map_info_btf_id(infoC)),
		BTFKeyTypeID:          uint32(C.cgo_bpf_map_info_btf_key_type_id(infoC)),
		BTFValueTypeID:        uint32(C.cgo_bpf_map_info_btf_value_type_id(infoC)),
		MapExtra:              uint64(C.cgo_bpf_map_info_map_extra(infoC)),
	}, nil
}

// CalcMapValueSize calculates the size of the value for a map.
// For per-CPU maps, it is calculated based on the number of possible CPUs.
func CalcMapValueSize(valueSize int, mapType MapType) (int, error) {
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
