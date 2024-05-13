package libbpfgo

/*
#cgo LDFLAGS: -lelf -lz
#include "libbpfgo.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

//
// BPFMapLow (low-level API)
//

// BPFMapLow provides a low-level interface to BPF maps.
// Its methods follow the BPFMap naming convention.
type BPFMapLow struct {
	fd   int
	info *BPFMapInfo
}

// BPFMapCreateOpts mirrors the C structure bpf_map_create_opts.
type BPFMapCreateOpts struct {
	BTFFD                 uint32
	BTFKeyTypeID          uint32
	BTFValueTypeID        uint32
	BTFVmlinuxValueTypeID uint32
	InnerMapFD            uint32
	MapFlags              uint32
	MapExtra              uint64
	NumaNode              uint32
	MapIfIndex            uint32
}

// CreateMap creates a new BPF map with the given parameters.
func CreateMap(mapType MapType, mapName string, keySize, valueSize, maxEntries int, opts *BPFMapCreateOpts) (*BPFMapLow, error) {
	mapNameC := C.CString(mapName)
	defer C.free(unsafe.Pointer(mapNameC))
	var optsC *C.struct_bpf_map_create_opts
	var errno error

	if opts != nil {
		optsC, errno = C.cgo_bpf_map_create_opts_new(
			C.uint(opts.BTFFD),
			C.uint(opts.BTFKeyTypeID),
			C.uint(opts.BTFValueTypeID),
			C.uint(opts.BTFVmlinuxValueTypeID),
			C.uint(opts.InnerMapFD),
			C.uint(opts.MapFlags),
			C.ulonglong(opts.MapExtra),
			C.uint(opts.NumaNode),
			C.uint(opts.MapIfIndex),
		)
		if optsC == nil {
			return nil, fmt.Errorf("failed to create bpf_map_create_opts: %w", errno)
		}
		defer C.cgo_bpf_map_create_opts_free(optsC)
	}

	fdC := C.bpf_map_create(uint32(mapType), mapNameC, C.uint(keySize), C.uint(valueSize), C.uint(maxEntries), optsC)
	if fdC < 0 {
		return nil, fmt.Errorf("could not create map %s: %w", mapName, syscall.Errno(-fdC))
	}

	info, errInfo := GetMapInfoByFD(int(fdC))
	if errInfo != nil {
		if errClose := syscall.Close(int(fdC)); errClose != nil {
			return nil, fmt.Errorf("could not create map %s: %w, %v", mapName, errInfo, errClose)
		}

		return nil, fmt.Errorf("could not create map %s: %w", mapName, errInfo)
	}

	return &BPFMapLow{
		fd:   int(fdC),
		info: info,
	}, nil
}

// GetMapByID returns a BPFMapLow instance for the map with the given ID.
func GetMapByID(id uint32) (*BPFMapLow, error) {
	fd, err := GetMapFDByID(id)
	if err != nil {
		return nil, err
	}

	info, err := GetMapInfoByFD(fd)
	if err != nil {
		return nil, err
	}

	return &BPFMapLow{
		fd:   fd,
		info: info,
	}, nil
}

// GetMapNextID retrieves the next available map ID after the given startID.
// It returns the next map ID and an error if one occurs during the operation.
func GetMapNextID(startId uint32) (uint32, error) {
	startIDC := C.uint(startId)
	retC := C.bpf_map_get_next_id(startIDC, &startIDC)
	if retC == 0 {
		return uint32(startIDC), nil
	}

	return uint32(startIDC), fmt.Errorf("failed to get next map id: %w", syscall.Errno(-retC))
}

// GetMapsIDsByName searches for maps with a specified name and collects their IDs.
// It starts the search from the given 'startId' and continues until no more matching maps are found.
// The function returns a slice of unsigned 32-bit integers representing the IDs of matching maps.
// If no maps with the provided 'name' are found, it returns an empty slice and no error.
// The 'startId' is modified and returned as the last processed map ID.
//
// Example Usage:
//
//	name := "myMap"          // The name of the map you want to find.
//	startId := uint32(0)     // The map ID to start the search from.
//
//	var mapIDs []uint32      // Initialize an empty slice to collect map IDs.
//	var err error            // Initialize an error variable.
//
//	// Retry mechanism in case of errors using the last processed 'startId'.
//	for {
//	    mapIDs, err = GetMapsIDsByName(name, startId)
//	    if err != nil {
//	        // Handle other errors, possibly with a retry mechanism.
//	        // You can use the 'startId' who contains the last processed map ID to continue the search.
//	    } else {
//	        // Successful search, use the 'mapIDs' slice containing the IDs of matching maps.
//	        // Update 'startId' to the last processed map ID to continue the search.
//	    }
//	}
func GetMapsIDsByName(name string, startId *uint32) ([]uint32, error) {
	var (
		bpfMapsIds []uint32
		err        error
	)

	for {
		*startId, err = GetMapNextID(*startId)
		if err != nil {
			if errors.Is(err, syscall.ENOENT) {
				return bpfMapsIds, nil
			}

			return bpfMapsIds, err
		}

		bpfMapLow, err := GetMapByID(*startId)
		if err != nil {
			return bpfMapsIds, err
		}

		if err := syscall.Close(bpfMapLow.FileDescriptor()); err != nil {
			return bpfMapsIds, err
		}

		if bpfMapLow.Name() != name {
			continue
		}

		bpfMapsIds = append(bpfMapsIds, bpfMapLow.info.ID)
	}
}

//
// BPFMapLow Specs
//

func (m *BPFMapLow) FileDescriptor() int {
	return m.fd
}

func (m *BPFMapLow) ReuseFD(fd int) error {
	info, err := GetMapInfoByFD(fd)
	if err != nil {
		return fmt.Errorf("failed to reuse fd %d: %w", fd, err)
	}

	newFD, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("failed to reuse fd %d: %w", fd, err)
	}

	err = syscall.Dup3(fd, newFD, syscall.O_CLOEXEC)
	if err != nil {
		_ = syscall.Close(newFD)
		return fmt.Errorf("failed to reuse fd %d: %w", fd, err)
	}

	err = syscall.Close(m.FileDescriptor())
	if err != nil {
		_ = syscall.Close(newFD)
		return fmt.Errorf("failed to reuse fd %d: %w", fd, err)
	}

	m.fd = newFD
	m.info = info

	return nil
}

func (m *BPFMapLow) Name() string {
	return m.info.Name
}

func (m *BPFMapLow) Type() MapType {
	return MapType(m.info.Type)
}

func (m *BPFMapLow) MaxEntries() uint32 {
	return m.info.MaxEntries
}

// TODO: implement `bpf_map__map_flags`
// func (m *BPFMapLow) MapFlags() MapFlag {
// }

// TODO: implement `bpf_map__numa_node`
// func (m *BPFMapLow) NUMANode() uint32 {
// }

func (m *BPFMapLow) KeySize() int {
	return int(m.info.KeySize)
}

func (m *BPFMapLow) ValueSize() int {
	return int(m.info.ValueSize)
}

// TODO: implement `bpf_map__btf_key_type_id`
// func (m *BPFMapLow) BTFKeyTypeID() uint32 {
// }

// TODO: implement `bpf_map__btf_value_type_id`
// func (m *BPFMapLow) BTFValueTypeID() uint32 {
// }

// TODO: implement `bpf_map__ifindex`
// func (m *BPFMapLow) IfIndex() uint32 {
// }

// TODO: implement `bpf_map__map_extra`
// func (m *BPFMapLow) MapExtra() uint64 {
// }

//
// BPFMapLow Operations
//

func (m *BPFMapLow) GetValue(key unsafe.Pointer) ([]byte, error) {
	return m.GetValueFlags(key, MapFlagUpdateAny)
}

func (m *BPFMapLow) GetValueFlags(key unsafe.Pointer, flags MapFlag) ([]byte, error) {
	valueSize, err := calcMapValueSize(m.ValueSize(), m.Type())
	if err != nil {
		return nil, fmt.Errorf("map %s %w", m.Name(), err)
	}

	value := make([]byte, valueSize)
	retC := C.bpf_map_lookup_elem_flags(
		C.int(m.FileDescriptor()),
		key,
		unsafe.Pointer(&value[0]),
		C.ulonglong(flags),
	)
	if retC < 0 {
		return nil, fmt.Errorf("failed to lookup value %v in map %s: %w", key, m.Name(), syscall.Errno(-retC))
	}

	return value, nil
}

// TODO: implement `bpf_map__lookup_and_delete_elem`
// func (m *BPFMapLow) GetValueAndDeleteKey(key unsafe.Pointer) ([]byte, error) {
// }

func (m *BPFMapLow) Update(key, value unsafe.Pointer) error {
	return m.UpdateValueFlags(key, value, MapFlagUpdateAny)
}

func (m *BPFMapLow) UpdateValueFlags(key, value unsafe.Pointer, flags MapFlag) error {
	retC := C.bpf_map_update_elem(
		C.int(m.FileDescriptor()),
		key,
		value,
		C.ulonglong(flags),
	)
	if retC < 0 {
		return fmt.Errorf("failed to update map %s: %w", m.Name(), syscall.Errno(-retC))
	}

	return nil
}

func (m *BPFMapLow) DeleteKey(key unsafe.Pointer) error {
	retC := C.bpf_map_delete_elem(C.int(m.FileDescriptor()), key)
	if retC < 0 {
		return fmt.Errorf("failed to delete key %d in map %s: %w", key, m.Name(), syscall.Errno(-retC))
	}

	return nil
}

// TODO: implement `bpf_map__get_next_key`
// func (m *BPFMapLow) GetNextKey(key unsafe.Pointer) (unsafe.Pointer, error) {
// }

//
// BPFMapLow Batch Operations
//

func (m *BPFMapLow) GetValueBatch(keys unsafe.Pointer, startKey, nextKey unsafe.Pointer, count uint32) ([][]byte, error) {
	valueSize, err := calcMapValueSize(m.ValueSize(), m.Type())
	if err != nil {
		return nil, fmt.Errorf("map %s %w", m.Name(), err)
	}

	var (
		values    = make([]byte, valueSize*int(count))
		valuesPtr = unsafe.Pointer(&values[0])
		countC    = C.uint(count)
	)

	optsC, errno := C.cgo_bpf_map_batch_opts_new(C.BPF_ANY, C.BPF_ANY)
	if optsC == nil {
		return nil, fmt.Errorf("failed to create bpf_map_batch_opts: %w", errno)
	}
	defer C.cgo_bpf_map_batch_opts_free(optsC)

	// The batch APIs are a bit different in which they can return an error, but
	// depending on the errno code, it might mean a complete error (nothing was
	// done) or a partial success (some elements were processed).
	//
	// - On complete sucess, it will return 0, and errno won't be set.
	// - On partial sucess, it will return -1, and errno will be set to ENOENT.
	// - On error, it will return -1, and an errno different to ENOENT.
	retC := C.bpf_map_lookup_batch(
		C.int(m.FileDescriptor()),
		startKey,
		nextKey,
		keys,
		valuesPtr,
		&countC,
		optsC,
	)
	errno = syscall.Errno(-retC)
	if retC < 0 && errno != syscall.ENOENT {
		return nil, fmt.Errorf("failed to batch get value %v in map %s: %w", keys, m.Name(), errno)
	}

	// Either some or all entries were read.
	// retC < 0 && errno == syscall.ENOENT indicates a partial read.
	return collectBatchValues(values, uint32(countC), valueSize), nil
}

func (m *BPFMapLow) GetValueAndDeleteBatch(keys, startKey, nextKey unsafe.Pointer, count uint32) ([][]byte, error) {
	valueSize, err := calcMapValueSize(m.ValueSize(), m.Type())
	if err != nil {
		return nil, fmt.Errorf("map %s %w", m.Name(), err)
	}

	var (
		values    = make([]byte, valueSize*int(count))
		valuesPtr = unsafe.Pointer(&values[0])
		countC    = C.uint(count)
	)

	optsC, errno := C.cgo_bpf_map_batch_opts_new(C.BPF_ANY, C.BPF_ANY)
	if optsC == nil {
		return nil, fmt.Errorf("failed to create bpf_map_batch_opts: %w", errno)
	}
	defer C.cgo_bpf_map_batch_opts_free(optsC)

	retC := C.bpf_map_lookup_and_delete_batch(
		C.int(m.FileDescriptor()),
		startKey,
		nextKey,
		keys,
		valuesPtr,
		&countC,
		optsC,
	)
	errno = syscall.Errno(-retC)
	if retC < 0 && errno != syscall.ENOENT {
		return nil, fmt.Errorf("failed to batch lookup and delete values %v in map %s: %w", keys, m.Name(), errno)
	}

	// Either some or all entries were read and deleted.
	// retC < 0 && errno == syscall.ENOENT indicates a partial read and delete.
	return collectBatchValues(values, uint32(countC), valueSize), nil
}

func (m *BPFMapLow) UpdateBatch(keys, values unsafe.Pointer, count uint32) error {
	countC := C.uint(count)

	optsC, errno := C.cgo_bpf_map_batch_opts_new(C.BPF_ANY, C.BPF_ANY)
	if optsC == nil {
		return fmt.Errorf("failed to create bpf_map_batch_opts: %w", errno)
	}
	defer C.cgo_bpf_map_batch_opts_free(optsC)

	retC := C.bpf_map_update_batch(
		C.int(m.FileDescriptor()),
		keys,
		values,
		&countC,
		optsC,
	)
	errno = syscall.Errno(-retC)
	if retC < 0 {
		if errno != syscall.EFAULT && uint32(countC) != count {
			return fmt.Errorf("failed to update ALL elements in map %s, updated (%d/%d): %w", m.Name(), uint32(countC), count, errno)
		}
		return fmt.Errorf("failed to batch update elements in map %s: %w", m.Name(), errno)
	}

	return nil
}

func (m *BPFMapLow) DeleteKeyBatch(keys unsafe.Pointer, count uint32) error {
	countC := C.uint(count)

	optsC, errno := C.cgo_bpf_map_batch_opts_new(C.BPF_ANY, C.BPF_ANY)
	if optsC == nil {
		return fmt.Errorf("failed to create bpf_map_batch_opts: %w", errno)
	}
	defer C.cgo_bpf_map_batch_opts_free(optsC)

	retC := C.bpf_map_delete_batch(
		C.int(m.FileDescriptor()),
		keys,
		&countC,
		optsC,
	)
	errno = syscall.Errno(-retC)
	if retC < 0 && errno != syscall.ENOENT {
		return fmt.Errorf("failed to batch delete keys %v in map %s: %w", keys, m.Name(), errno)
	}

	// retC < 0 && errno == syscall.ENOENT indicates a partial deletion.
	return nil
}

func collectBatchValues(values []byte, count uint32, valueSize int) [][]byte {
	var value []byte
	var collected [][]byte

	for i := 0; i < int(count*uint32(valueSize)); i += valueSize {
		value = values[i : i+valueSize]
		collected = append(collected, value)
	}

	return collected
}

//
// BPFMapLow Iterator
//

func (m *BPFMapLow) Iterator() *BPFMapIterator {
	return &BPFMapIterator{
		mapFD:   m.FileDescriptor(),
		keySize: m.KeySize(),
		prev:    nil,
		next:    nil,
	}
}
