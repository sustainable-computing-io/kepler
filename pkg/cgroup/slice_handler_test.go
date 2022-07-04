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

package cgroup

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"strings"
	"fmt"
)

const (
	SLICE_SUFFIX = ".slice"
	SCOPE_SUFFIX = ".scope"
)

var testPaths []string = []string {
	"./test/hierarchypath", "./test/toppath/kubepod", "./test/toppath/system",
}

var expectedStandardStats map[string]int = map[string]int{
	testPaths[0]: 6,
	testPaths[1]: 6,
	testPaths[2]: 6,
}

func initSliceHandler(basePath string) *SliceHandler {
	BASE_CGROUP_PATH = basePath
	KUBEPOD_CGROUP_PATH = fmt.Sprintf("%s/kubepods.slice", basePath)
	SYSTEM_CGROUP_PATH = fmt.Sprintf("%s/system.slice", basePath)
	sliceHandler := InitSliceHandler()
	return sliceHandler

}

func findContainerScope(path string) string {
	if strings.Contains(path, SCOPE_SUFFIX) {
		return path
	}
	slicePath := SearchByContainerID(path, SLICE_SUFFIX)
	if slicePath == "" {
		return ""
	}
	return findContainerScope(slicePath)
}

func findExampleContainerID(slice *SliceHandler) string {
	topPath := slice.GetCPUTopPath()
	containerScopePath := findContainerScope(topPath)
	pathSplits := strings.Split(containerScopePath, "/")
	fileName := pathSplits[len(pathSplits) - 1]
	scopeSplit := strings.Split(fileName, ".scope")[0]
	partSplits := strings.Split(scopeSplit, "-")
	return partSplits[len(partSplits)-1]
}


var _ = Describe("Test Read Stat", func() {
	It("Properly find container path", func() {
		for _, testPath := range testPaths {
			SliceHandlerInstance = initSliceHandler(testPath)
			containerID := findExampleContainerID(SliceHandlerInstance)
			Expect(containerID).NotTo(Equal(""))
		}
	})

	It("Properly read stat", func() {
		for _, testPath := range testPaths {
			SliceHandlerInstance = initSliceHandler(testPath)
			containerID := findExampleContainerID(SliceHandlerInstance)
			Expect(containerID).NotTo(Equal(""))
			TryInitStatReaders(containerID)
			statReaders := SliceHandlerInstance.GetStatReaders()
			Expect(len(statReaders)).To(Equal(1))
			fmt.Println(statReaders)
			stats := SliceHandlerInstance.GetStats(containerID)
			fmt.Println(stats)
			Expect(len(stats)).Should(BeNumerically(">", 0))
		}
	})

	It("Properly read standard stats", func() {
		for _, testPath := range testPaths {
			SliceHandlerInstance = initSliceHandler(testPath)
			containerID := findExampleContainerID(SliceHandlerInstance)
			Expect(containerID).NotTo(Equal(""))
			TryInitStatReaders(containerID)
			standardStats := GetStandardStat(containerID)
			fmt.Println(standardStats)
			Expect(len(standardStats)).To(Equal(expectedStandardStats[testPath]))
		}
	})
})
