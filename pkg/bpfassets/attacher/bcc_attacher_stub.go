//go:build !bcc
// +build !bcc

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package attacher

import (
	"fmt"
)

type perfCounter struct{}

type ModuleStub struct{}

// Table references a BPF table.
type Table struct {
}

// TableIterator contains the current position for iteration over a *bcc.Table and provides methods for iteration.
type TableIterator struct {
	leaf []byte
	key  []byte
}

// Iter returns an iterator to list all table entries available as raw bytes.
func (table *Table) Iter() *TableIterator {
	return &TableIterator{}
}

// Next looks up the next element and return true if one is available.
func (it *TableIterator) Next() bool {
	return false
}

// Key returns the current key value of the iterator, if the most recent call to Next returned true.
// The slice is valid only until the next call to Next.
func (it *TableIterator) Key() []byte {
	return it.key
}

// Leaf returns the current leaf value of the iterator, if the most recent call to Next returned true.
// The slice is valid only until the next call to Next.
func (it *TableIterator) Leaf() []byte {
	return it.leaf
}

// ID returns the table id.
func (table *Table) ID() string {
	return ""
}

// Delete a key.
func (table *Table) Delete(key []byte) error {
	return nil
}

func (table *Table) DeleteAll() {
}

func TableDeleteBatch(module ModuleStub, tableName string, keys [][]byte) error {
	return nil
}

type BpfModuleTables struct {
	Module       ModuleStub
	Table        *Table
	TableName    string
	CPUFreqTable *Table
}

var (
	Counters                = map[string]perfCounter{}
	HardwareCountersEnabled = true
)

func AttachBPFAssets() (*BpfModuleTables, error) {
	return nil, fmt.Errorf("no bcc build tag")
}

func DetachBPFModules(bpfModules *BpfModuleTables) {
}

func GetEnabledHWCounters() []string {
	return []string{}
}

func GetEnabledBPFCounters() []string {
	return []string{}
}
