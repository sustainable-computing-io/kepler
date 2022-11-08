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

const (
	CPUCycleLable       = "cpu_cycles"
	CPUInstructionLabel = "cpu_instr"
	CacheMissLabel      = "cache_miss"
)

type perfCounter struct{}

type ModuleStub struct{}

type Table struct {
}
type TableIterator struct {
	leaf []byte
}

func (table *Table) Iter() *TableIterator {
	return &TableIterator{}
}

func (it *TableIterator) Next() bool {
	return false
}

func (it *TableIterator) Leaf() []byte {
	return it.leaf
}

func (table *Table) DeleteAll() {
}

type BpfModuleTables struct {
	Module    ModuleStub
	Table     *Table
	TimeTable *Table
}

var (
	Counters      = map[string]perfCounter{}
	EnableCPUFreq = false
)

func AttachBPFAssets() (*BpfModuleTables, error) {
	return nil, fmt.Errorf("no bcc build tag")
}

func DetachBPFModules(bpfModules *BpfModuleTables) {
}

func GetEnabledCounters() []string {
	return []string{}
}
