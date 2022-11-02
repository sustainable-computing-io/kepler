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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var testPaths []string = []string{
	"./test/hierarchypath", "./test/toppath/kubepod", "./test/toppath/system",
}

var expectedStandardStats map[string]int = map[string]int{
	testPaths[0]: 6,
	testPaths[1]: 6,
	testPaths[2]: 6,
}

func initSliceHandler(basePath string) *SliceHandler {
	baseCGroupPath = basePath
	KubePodCGroupPath = fmt.Sprintf("%s/kubepods.slice", basePath)
	SystemCGroupPath = fmt.Sprintf("%s/system.slice", basePath)
	sliceHandler := InitSliceHandler()
	return sliceHandler
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

	It("Properly get available stats", func() {
		for _, testPath := range testPaths {
			SliceHandlerInstance = initSliceHandler(testPath)
			availableMetrics := GetAvailableCgroupMetrics()
			Expect(len(availableMetrics)).Should(BeNumerically(">", 0))
			fmt.Println("Available Metrics:", availableMetrics)
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
