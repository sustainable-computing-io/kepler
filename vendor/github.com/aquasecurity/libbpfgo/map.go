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
// BPFMap (high-level API - `bpf_map__*`)
//

// BPFMap is a wrapper around a libbpf bpf_map.
type BPFMap struct {
	bpfMap    *C.struct_bpf_map
	bpfMapLow *BPFMapLow
	module    *Module
}

//
// BPFMap Specs
//

func (m *BPFMap) Module() *Module {
	return m.module
}

// Deprecated: use BPFMap.Module() instead.
func (m *BPFMap) GetModule() *Module {
	return m.Module()
}

func (m *BPFMap) FileDescriptor() int {
	return int(C.bpf_map__fd(m.bpfMap))
}

// Deprecated: use BPFMap.FileDescriptor() instead.
func (m *BPFMap) GetFd() int {
	return m.FileDescriptor()
}

// ReuseFD associates the BPFMap instance with the provided map file descriptor.
//
// This function is useful for reusing a map that was previously created by a
// different process. By passing the file descriptor of the existing map, the
// current BPFMap instance becomes linked to that map.
//
// NOTE: The function closes the current file descriptor associated with the
// BPFMap instance and replaces it with a duplicated descriptor pointing to the
// given fd. As a result, the instance original file descriptor becomes invalid,
// and all associated information is overwritten.
func (m *BPFMap) ReuseFD(fd int) error {
	retC := C.bpf_map__reuse_fd(m.bpfMap, C.int(fd))
	if retC < 0 {
		return fmt.Errorf("failed to reuse fd %d: %w", fd, syscall.Errno(-retC))
	}

	newFD := m.FileDescriptor()
	info, err := GetMapInfoByFD(newFD)
	if err != nil {
		return err
	}

	m.bpfMapLow.fd = newFD
	m.bpfMapLow.info = info

	return nil
}

func (m *BPFMap) Name() string {
	return C.GoString(C.bpf_map__name(m.bpfMap))
}

// Deprecated: use BPFMap.Name() instead.
func (m *BPFMap) GetName() string {
	return m.Name()
}

func (m *BPFMap) Type() MapType {
	return MapType(C.bpf_map__type(m.bpfMap))
}

// SetType assigns a specific type to a BPFMap instance that is not yet associated
// with a file descriptor.
func (m *BPFMap) SetType(mapType MapType) error {
	retC := C.bpf_map__set_type(m.bpfMap, C.enum_bpf_map_type(int(mapType)))
	if retC < 0 {
		return fmt.Errorf("could not set bpf map type: %w", syscall.Errno(-retC))
	}

	return nil
}

// MaxEntries returns the capacity of the BPFMap.
//
// For ring and perf buffer types, this returns the capacity in bytes.
func (m *BPFMap) MaxEntries() uint32 {
	return uint32(C.bpf_map__max_entries(m.bpfMap))
}

// Deprecated: use BPFMap.MaxEntries() instead.
func (m *BPFMap) GetMaxEntries() uint32 {
	return m.MaxEntries()
}

// SetMaxEntries sets the capacity of the BPFMap to the given maxEntries value.
//
// This function must be called after BPF module initialization and before loading
// the module with BPFLoadObject, enabling customization of the map capacity.
//
// For ring and perf buffer types, maxEntries represents the capacity in bytes.
func (m *BPFMap) SetMaxEntries(maxEntries uint32) error {
	retC := C.bpf_map__set_max_entries(m.bpfMap, C.uint(maxEntries))
	if retC < 0 {
		return fmt.Errorf("failed to set map %s max entries to %v: %w", m.Name(), maxEntries, syscall.Errno(-retC))
	}

	return nil
}

// Deprecated: use BPFMap.SetMaxEntries() instead.
func (m *BPFMap) Resize(maxEntries uint32) error {
	return m.SetMaxEntries(maxEntries)
}

func (m *BPFMap) MapFlags() MapFlag {
	return MapFlag(C.bpf_map__map_flags(m.bpfMap))
}

// TODO: implement `bpf_map__set_map_flags` wrapper
// func (m *BPFMap) SetMapFlags(flags MapFlag) error {
// }

// TODO: implement `bpf_map__numa_node` wrapper
// func (m *BPFMap) NUMANode() uint32 {
// }

// TODO: implement `bpf_map__set_numa_node` wrapper
// func (m *BPFMap) SetNUMANode(node uint32) error {
// }

func (m *BPFMap) KeySize() int {
	return int(C.bpf_map__key_size(m.bpfMap))
}

// SetKeySize sets the key size to a BPFMap instance that is not yet associated
// with a file descriptor.
func (m *BPFMap) SetKeySize(size uint32) error {
	retC := C.bpf_map__set_key_size(m.bpfMap, C.uint(size))
	if retC < 0 {
		return fmt.Errorf("could not set map key size: %w", syscall.Errno(-retC))
	}

	return nil
}

func (m *BPFMap) ValueSize() int {
	return int(C.bpf_map__value_size(m.bpfMap))
}

// SetValueSize sets the value size to a BPFMap instance that is not yet associated
// with a file descriptor.
func (m *BPFMap) SetValueSize(size uint32) error {
	retC := C.bpf_map__set_value_size(m.bpfMap, C.uint(size))
	if retC < 0 {
		return fmt.Errorf("could not set map value size: %w", syscall.Errno(-retC))
	}

	return nil
}

func (m *BPFMap) Autocreate() bool {
	return bool(C.bpf_map__autocreate(m.bpfMap))
}

// Autocreate sets whether libbpf has to auto-create BPF map during BPF object
// load phase.
func (m *BPFMap) SetAutocreate(autocreate bool) error {
	retC := C.bpf_map__set_autocreate(m.bpfMap, C.bool(autocreate))
	if retC < 0 {
		return fmt.Errorf("could not set map autocreate: %w", syscall.Errno(-retC))
	}

	return nil
}

func (m *BPFMap) BTFKeyTypeID() uint32 {
	return uint32(C.bpf_map__btf_key_type_id(m.bpfMap))
}

func (m *BPFMap) BTFValueTypeID() uint32 {
	return uint32(C.bpf_map__btf_value_type_id(m.bpfMap))
}

func (m *BPFMap) IfIndex() uint32 {
	return uint32(C.bpf_map__ifindex(m.bpfMap))
}

// TODO: implement `bpf_map__set_ifindex` wrapper
// func (m *BPFMap) SetIfIndex(ifIndex uint32) error {
// }

func (m *BPFMap) MapExtra() uint64 {
	return uint64(C.bpf_map__map_extra(m.bpfMap))
}

// TODO: implement `bpf_map__set_map_extra` wrapper
// func (m *BPFMap) SetMapExtra(extra uint64) error {
// }

func (m *BPFMap) InitialValue() ([]byte, error) {
	valueSize, err := CalcMapValueSize(m.ValueSize(), m.Type())
	if err != nil {
		return nil, fmt.Errorf("map %s %w", m.Name(), err)
	}

	value := make([]byte, valueSize)
	C.cgo_bpf_map__initial_value(m.bpfMap, unsafe.Pointer(&value[0]))

	return value, nil
}

func (m *BPFMap) SetInitialValue(value unsafe.Pointer) error {
	valueSize, err := CalcMapValueSize(m.ValueSize(), m.Type())
	if err != nil {
		return fmt.Errorf("map %s %w", m.Name(), err)
	}

	retC := C.bpf_map__set_initial_value(m.bpfMap, value, C.ulong(valueSize))
	if retC < 0 {
		return fmt.Errorf("failed to set inital value for map %s: %w", m.Name(), syscall.Errno(-retC))
	}

	return nil
}

// TODO: implement `bpf_map__is_internal` wrapper
// func (m *BPFMap) IsInternal() bool {
// }

//
// BPFMap Pinning
//

func (m *BPFMap) PinPath() string {
	return C.GoString(C.bpf_map__pin_path(m.bpfMap))
}

// Deprecated: use BPFMap.PinPath() instead.
func (m *BPFMap) GetPinPath() string {
	return m.PinPath()
}

func (m *BPFMap) SetPinPath(pinPath string) error {
	pathC := C.CString(pinPath)
	defer C.free(unsafe.Pointer(pathC))

	retC := C.bpf_map__set_pin_path(m.bpfMap, pathC)
	if retC < 0 {
		return fmt.Errorf("failed to set pin for map %s to path %s: %w", m.Name(), pinPath, syscall.Errno(-retC))
	}

	return nil
}

func (m *BPFMap) IsPinned() bool {
	return bool(C.bpf_map__is_pinned(m.bpfMap))
}

func (m *BPFMap) Pin(pinPath string) error {
	pathC := C.CString(pinPath)
	defer C.free(unsafe.Pointer(pathC))

	retC := C.bpf_map__pin(m.bpfMap, pathC)
	if retC < 0 {
		return fmt.Errorf("failed to pin map %s to path %s: %w", m.Name(), pinPath, syscall.Errno(-retC))
	}

	return nil
}

func (m *BPFMap) Unpin(pinPath string) error {
	pathC := C.CString(pinPath)
	defer C.free(unsafe.Pointer(pathC))

	retC := C.bpf_map__unpin(m.bpfMap, pathC)
	if retC < 0 {
		return fmt.Errorf("failed to unpin map %s from path %s: %w", m.Name(), pinPath, syscall.Errno(-retC))
	}

	return nil
}

//
// BPFMap Map of Maps
//

// InnerMap retrieves the inner map prototype information associated with a
// BPFMap that represents a map of maps.
//
// NOTE: It must be called before the module is loaded, since it is a prototype
// destroyed right after the outer map is created.
//
// Reference:
// https://lore.kernel.org/bpf/20200429002739.48006-4-andriin@fb.com/
func (m *BPFMap) InnerMapInfo() (*BPFMapInfo, error) {
	innerMapC, errno := C.bpf_map__inner_map(m.bpfMap)
	if innerMapC == nil {
		return nil, fmt.Errorf("failed to get inner map for %s: %w", m.Name(), errno)
	}

	innerBPFMap := &BPFMap{
		bpfMap: innerMapC,
		module: m.module,
	}

	return &BPFMapInfo{
		// as it is a prototype, some values are not available
		Type:                  innerBPFMap.Type(),
		ID:                    0,
		KeySize:               uint32(innerBPFMap.KeySize()),
		ValueSize:             uint32(innerBPFMap.ValueSize()),
		MaxEntries:            innerBPFMap.MaxEntries(),
		MapFlags:              uint32(innerBPFMap.MapFlags()),
		Name:                  innerBPFMap.Name(),
		IfIndex:               innerBPFMap.IfIndex(),
		BTFVmlinuxValueTypeID: 0,
		NetnsDev:              0,
		NetnsIno:              0,
		BTFID:                 0,
		BTFKeyTypeID:          innerBPFMap.BTFKeyTypeID(),
		BTFValueTypeID:        innerBPFMap.BTFValueTypeID(),
		MapExtra:              innerBPFMap.MapExtra(),
	}, nil
}

// SetInnerMap configures the inner map prototype for a BPFMap that represents
// a map of maps.
//
// This function accepts the file descriptor of another map, which will serve as
// a prototype.
//
// NOTE: It must be called before the module is loaded.
func (m *BPFMap) SetInnerMap(templateMapFD int) error {
	if templateMapFD < 0 {
		return fmt.Errorf("invalid inner map fd %d", templateMapFD)
	}

	retC := C.bpf_map__set_inner_map_fd(m.bpfMap, C.int(templateMapFD))
	if retC < 0 {
		return fmt.Errorf("failed to set inner map for %s: %w", m.Name(), syscall.Errno(-retC))
	}

	return nil
}

//
// BPFMap Operations
//

// GetValue retrieves the value associated with a given key in the BPFMap.
//
// This function accepts an unsafe.Pointer to the key value to be searched
// in the map, and it returns the corresponding value as a slice of bytes.
// All basic types, and structs are supported as keys.
//
// NOTE: Slices and arrays are supported, but references should point to the first
// element in the slice or array, instead of the slice or array itself. This is
// crucial to prevent undefined behavior.
//
// For example:
//
// key := []byte{'a', 'b', 'c'}
// keyPtr := unsafe.Pointer(&key[0])
// bpfmap.GetValue(keyPtr)
func (m *BPFMap) GetValue(key unsafe.Pointer) ([]byte, error) {
	return m.GetValueFlags(key, MapFlagUpdateAny)
}

func (m *BPFMap) GetValueFlags(key unsafe.Pointer, flags MapFlag) ([]byte, error) {
	valueSize, err := CalcMapValueSize(m.ValueSize(), m.Type())
	if err != nil {
		return nil, fmt.Errorf("map %s %w", m.Name(), err)
	}

	value := make([]byte, valueSize)
	retC := C.bpf_map__lookup_elem(
		m.bpfMap,
		key,
		C.ulong(m.KeySize()),
		unsafe.Pointer(&value[0]),
		C.ulong(valueSize),
		C.ulonglong(flags),
	)
	if retC < 0 {
		return nil, fmt.Errorf("failed to lookup value %v in map %s: %w", key, m.Name(), syscall.Errno(-retC))
	}

	return value, nil
}

// LookupAndDeleteElem stores the value associated with a given key into the
// provided unsafe.Pointer and deletes the key from the BPFMap.
func (m *BPFMap) LookupAndDeleteElem(
	key unsafe.Pointer,
	value unsafe.Pointer,
	valueSize uint64,
	flags MapFlag,
) error {
	retC := C.bpf_map__lookup_and_delete_elem(
		m.bpfMap,
		key,
		C.ulong(m.KeySize()),
		value,
		C.ulong(valueSize),
		C.ulonglong(flags),
	)
	if retC < 0 {
		return fmt.Errorf("failed to lookup and delete value %v in map %s: %w", key, m.Name(), syscall.Errno(-retC))
	}

	return nil
}

// GetValueAndDeleteKey retrieves the value associated with a given key
// and delete the key in the BPFMap.
// It returns the value as a slice of bytes.
func (m *BPFMap) GetValueAndDeleteKey(key unsafe.Pointer) ([]byte, error) {
	return m.GetValueAndDeleteKeyFlags(key, MapFlagUpdateAny)
}

// GetValueAndDeleteKeyFlags retrieves the value associated with a given key
// and delete the key in the BPFMap, with the specified flags.
// It returns the value as a slice of bytes.
func (m *BPFMap) GetValueAndDeleteKeyFlags(key unsafe.Pointer, flags MapFlag) ([]byte, error) {
	valueSize, err := CalcMapValueSize(m.ValueSize(), m.Type())
	if err != nil {
		return nil, fmt.Errorf("map %s %w", m.Name(), err)
	}

	value := make([]byte, valueSize)
	err = m.LookupAndDeleteElem(key, unsafe.Pointer(&value[0]), uint64(valueSize), flags)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Deprecated: use BPFMap.GetValue() or BPFMap.GetValueFlags() instead, since
// they already calculate the value size for per-cpu maps.
func (m *BPFMap) GetValueReadInto(key unsafe.Pointer, value *[]byte) error {
	valuePtr := unsafe.Pointer(&(*value)[0])
	retC := C.bpf_map__lookup_elem(m.bpfMap, key, C.ulong(m.KeySize()), valuePtr, C.ulong(len(*value)), 0)
	if retC < 0 {
		return fmt.Errorf("failed to lookup value %v in map %s: %w", key, m.Name(), syscall.Errno(-retC))
	}

	return nil
}

// Update inserts or updates value in BPFMap that corresponds to a given key.
//
// This function accepts unsafe.Pointer references to both the key and value.
// All basic types, and structs are supported.
//
// NOTE: Slices and arrays are supported, but references should point to the first
// element in the slice or array, instead of the slice or array itself. This is
// crucial to prevent undefined behavior.
//
// For example:
//
// key := 1
// value := []byte{'a', 'b', 'c'}
// keyPtr := unsafe.Pointer(&key)
// valuePtr := unsafe.Pointer(&value[0])
// bpfmap.Update(keyPtr, valuePtr)
func (m *BPFMap) Update(key, value unsafe.Pointer) error {
	return m.UpdateValueFlags(key, value, MapFlagUpdateAny)
}

func (m *BPFMap) UpdateValueFlags(key, value unsafe.Pointer, flags MapFlag) error {
	valueSize, err := CalcMapValueSize(m.ValueSize(), m.Type())
	if err != nil {
		return fmt.Errorf("map %s %w", m.Name(), err)
	}

	retC := C.bpf_map__update_elem(
		m.bpfMap,
		key,
		C.ulong(m.KeySize()),
		value,
		C.ulong(valueSize),
		C.ulonglong(flags),
	)
	if retC < 0 {
		return fmt.Errorf("failed to update map %s: %w", m.Name(), syscall.Errno(-retC))
	}

	return nil
}

// DeleteKey removes a specified key and its associated value from the BPFMap.
//
// This function accepts an unsafe.Pointer that references the key to be
// removed from the map.
// All basic types, and structs are supported as keys.
//
// NOTE: Slices and arrays are supported, but references should point to the first
// element in the slice or array, instead of the slice or array itself. This is
// crucial to prevent undefined behavior.
func (m *BPFMap) DeleteKey(key unsafe.Pointer) error {
	retC := C.bpf_map__delete_elem(m.bpfMap, key, C.ulong(m.KeySize()), 0)
	if retC < 0 {
		return fmt.Errorf("failed to delete key %d in map %s: %w", key, m.Name(), syscall.Errno(-retC))
	}

	return nil
}

// GetNextKey allows to iterate BPF map keys by fetching next key that follows current key.
func (m *BPFMap) GetNextKey(key unsafe.Pointer, nextKey unsafe.Pointer) error {
	retC := C.bpf_map__get_next_key(
		m.bpfMap,
		key,
		nextKey,
		C.ulong(m.KeySize()),
	)
	if retC < 0 {
		return fmt.Errorf("failed to get next key %d in map %s: %w", key, m.Name(), syscall.Errno(-retC))
	}

	return nil
}

//
// BPFMap Batch Operations (low-level API)
//

// GetValueBatch allows for batch lookups of multiple keys from the map.
//
// The first argument, keys, is a pointer to an array or slice of keys which will
// be populated with the keys returned from this operation.
//
// This API allows for batch lookups of multiple keys, potentially in steps over
// multiple iterations. For example, you provide the last key seen (or nil) for
// the startKey, and the first key to start the next iteration with in nextKey.
// Once the first iteration is complete you can provide the last key seen in the
// previous iteration as the startKey for the next iteration and repeat until
// nextKey is nil.
//
// The last argument, count, is the number of keys to lookup.
//
// It returns the associated values as a slice of slices of bytes and the number
// of elements that were retrieved.
//
// The API can return partial results even though the underlying logic received -1.
// In this case, no error will be returned. For checking if the returned values
// are partial, you can compare the number of elements returned with the passed
// count. See the comment in `BPFMapLow.GetValueBatch` for more context.
func (m *BPFMap) GetValueBatch(keys, startKey, nextKey unsafe.Pointer, count uint32) ([][]byte, uint32, error) {
	return m.bpfMapLow.GetValueBatch(keys, startKey, nextKey, count)
}

// GetValueAndDeleteBatch allows for batch lookup and deletion of elements where
// each element is deleted after being retrieved from the map.
//
// The first argument, keys, is a pointer to an array or slice of keys which will
// be populated with the keys returned from this operation.
//
// This API allows for batch lookups and deletion of multiple keys, potentially
// in steps over multiple iterations. For example, you provide the last key seen
// (or nil) for the startKey, and the first key to start the next iteration
// with in nextKey.
// Once the first iteration is complete you can provide the last key seen in the
// previous iteration as the startKey for the next iteration and repeat until
// nextKey is nil.
//
// The last argument, count, is the number of keys to lookup and delete.
//
// It returns the associated values as a slice of slices of bytes and the number
// of elements that were retrieved and deleted.
//
// The API can return partial results even though the underlying logic received -1.
// In this case, no error will be returned. For checking if the returned values
// are partial, you can compare the number of elements returned with the passed
// count. See the comment in `BPFMapLow.GetValueBatch` for more context.
func (m *BPFMap) GetValueAndDeleteBatch(keys, startKey, nextKey unsafe.Pointer, count uint32) ([][]byte, uint32, error) {
	return m.bpfMapLow.GetValueAndDeleteBatch(keys, startKey, nextKey, count)
}

// UpdateBatch updates multiple elements in the map by specified keys and their
// corresponding values.
//
// The first argument, keys, is a pointer to an array or slice of keys which will
// be updated using the second argument, values.
//
// The last argument, count, is the number of keys to update.
//
// It returns the number of elements that were updated.
//
// The API can update fewer elements than requested even though the underlying
// logic received -1. This can happen if the map is full and the update operation
// fails for some of the keys. In this case, no error will be returned. For
// checking if the updated values are partial, you can compare the number of
// elements returned with the passed count. See the comment in
// `BPFMapLow.GetValueBatch` and `BPFMapLow.UpdateBatch` for more context.
func (m *BPFMap) UpdateBatch(keys, values unsafe.Pointer, count uint32) (uint32, error) {
	return m.bpfMapLow.UpdateBatch(keys, values, count)
}

// DeleteKeyBatch deletes multiple elements from the map by specified keys.
//
// The first argument, keys, is a pointer to an array or slice of keys which will
// be deleted.
//
// The last argument, count, is the number of keys to delete.
//
// It returns the number of elements that were deleted.
//
// The API can delete fewer elements than requested even though the underlying
// logic received -1. For checking if the deleted elements are partial, you can
// compare the number of elements returned with the passed count. See the comment
// in `BPFMapLow.GetValueBatch` for more context.
func (m *BPFMap) DeleteKeyBatch(keys unsafe.Pointer, count uint32) (uint32, error) {
	return m.bpfMapLow.DeleteKeyBatch(keys, count)
}

//
// BPFMap Iterator (low-level API)
//

func (m *BPFMap) Iterator() *BPFMapIterator {
	return &BPFMapIterator{
		mapFD:   m.FileDescriptor(),
		keySize: m.KeySize(),
		prev:    nil,
		next:    nil,
	}
}
