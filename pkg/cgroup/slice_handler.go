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

/*

This file is a main file of cgroup module containing
- init
- TryInitStatReaders
- GetStandardStat

*/

package cgroup

import 	(
	"path/filepath"
	"os"
	"log"
	"strings"
)

var (
	BASE_CGROUP_PATH string = "/sys/fs/cgroup"
	KUBEPOD_CGROUP_PATH string = "/sys/fs/cgroup/kubepods.slice"
	SYSTEM_CGROUP_PATH string = "/sys/fs/cgroup/system.slice"
)

type SliceHandler struct {
	statReaders map[string][]StatReader
	CPUTopPath string
	MemoryTopPath string
	IOTopPath string
}

var SliceHandlerInstance *SliceHandler

func init() {
	SliceHandlerInstance = InitSliceHandler()
}

func (s *SliceHandler) Init() {
	s.statReaders = make(map[string][]StatReader)
}

func (s *SliceHandler) GetStatReaders() map[string][]StatReader {
	return s.statReaders
}

func (s *SliceHandler) SetStatReaders(containerID string, statReaders []StatReader) {
	s.statReaders[containerID] = statReaders
}

func (s *SliceHandler) GetCPUTopPath() string {
	return s.CPUTopPath
}

func (s *SliceHandler) GetMemoryTopPath() string {
	return s.MemoryTopPath
}

func (s *SliceHandler) GetIOTopPath() string {
	return s.IOTopPath
}

func (s *SliceHandler) GetStats(containerID string) map[string]interface{} {
	if readers, exists := s.statReaders[containerID]; exists {
		values := make(map[string]interface{})
		for _, reader := range readers {
			newValues := reader.Read()
			for k, v := range newValues {
				values[k] = v
			}
		}
		return values
	}
	return map[string]interface{}{}
}

func InitSliceHandler() *SliceHandler {
	var handler *SliceHandler
	if _, err := os.Stat(KUBEPOD_CGROUP_PATH); err == nil {
		handler = &SliceHandler{
			CPUTopPath: KUBEPOD_CGROUP_PATH,
			MemoryTopPath: KUBEPOD_CGROUP_PATH,
			IOTopPath: KUBEPOD_CGROUP_PATH,
		}
	} else if _, err := os.Stat(SYSTEM_CGROUP_PATH); err == nil {
		handler = &SliceHandler{
			CPUTopPath: SYSTEM_CGROUP_PATH,
			MemoryTopPath: SYSTEM_CGROUP_PATH,
			IOTopPath: SYSTEM_CGROUP_PATH,
		}
	} else {
		handler = &SliceHandler{
			CPUTopPath: filepath.Join(BASE_CGROUP_PATH, "cpu"),
			MemoryTopPath: filepath.Join(BASE_CGROUP_PATH, "memory"),
			IOTopPath: filepath.Join(BASE_CGROUP_PATH, "blkio"),
		}
	}
	handler.Init()
	log.Printf("InitSliceHandler: %v", handler)
	return handler
}

func TryInitStatReaders(containerID string) {
	statReaders := SliceHandlerInstance.GetStatReaders()
	if _, exists := statReaders[containerID]; !exists {
		cpuTopPath := SliceHandlerInstance.GetCPUTopPath()
		memoryTopPath := SliceHandlerInstance.GetMemoryTopPath()
		ioTopPath := SliceHandlerInstance.GetIOTopPath()
		containerCPUPath := SearchByContainerID(cpuTopPath, containerID)
		containerMemoryPath := strings.Replace(containerCPUPath, cpuTopPath, memoryTopPath, 1)
		containerIOPath := strings.Replace(containerCPUPath, cpuTopPath, ioTopPath, 1)
		statReaders[containerID] = []StatReader{
			CPUStatReader{ Path: containerCPUPath },
			MemoryStatReader{ Path: containerMemoryPath },
			IOStatReader { Path: containerIOPath },
		}
	}
}

func GetStandardStat(containerID string) map[string]interface{} {
	stats := SliceHandlerInstance.GetStats(containerID)
	return convertToStandard(stats)
}