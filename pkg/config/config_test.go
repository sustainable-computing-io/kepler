/*
Copyright 2022.

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

package config

import (
	"fmt"
	"os"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"
)

type mockconfig struct {
}

var mockc mockconfig
var tempfile string
var tempUnixName string

func (c mockconfig) getUnixName() (unix.Utsname, error) {
	var utsname unix.Utsname
	copy(utsname.Release[:], tempUnixName)
	return utsname, nil
}

func (c mockconfig) getCgroupV2File() string {
	return tempfile
}

func createTempFile(contents string) (filename string, reterr error) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}
	defer func() {
		if err := f.Close(); err != nil {
			return
		}
	}()
	_, _ = f.WriteString(contents)
	return f.Name(), nil
}

var _ = Describe("Test Configuration", func() {
	It("Test cgroup version", func() {
		file, err := createTempFile("")
		Expect(err).NotTo(HaveOccurred())
		defer os.Remove(file)
		tempfile = file

		Expect(true).To(Equal(isCGroupV2(mockc)))

		tempfile = "/tmp/this_file_do_not_exist"
		Expect(false).To(Equal(isCGroupV2(mockc)))
	})
	It("Test kernel version compare", func() {
		tempUnixName = "5.10.0-20-generic"
		v := getKernelVersion(mockc)
		valid := v > cGroupIDMinKernelVersion
		Expect(-1).NotTo(Equal(v))
		Expect(true).To(Equal(valid))

		tempUnixName = "5.4.0-20-generic"
		v = getKernelVersion(mockc)
		valid = v > cGroupIDMinKernelVersion
		Expect(-1).NotTo(Equal(v))
		Expect(true).To(Equal(valid))

		tempUnixName = "6.0-rc6"
		v = getKernelVersion(mockc)
		valid = v > cGroupIDMinKernelVersion
		Expect(-1).NotTo(Equal(v))
		Expect(true).To(Equal(valid))

		tempUnixName = "3.10"
		v = getKernelVersion(mockc)
		valid = v > cGroupIDMinKernelVersion
		Expect(-1).NotTo(Equal(v))
		Expect(false).To(Equal(valid))

		tempUnixName = "3.1"
		v = getKernelVersion(mockc)
		valid = v > cGroupIDMinKernelVersion
		Expect(-1).NotTo(Equal(v))
		Expect(false).To(Equal(valid))
	})
	It("Test not able to detect kernel", func() {
		tempUnixName = "dummy_test_result_not.found"
		Expect(float32(-1)).To(Equal(getKernelVersion(mockc)))
	})
	It("Test real kernel version", func() {
		// we assume running on Linux env should be bigger than 3.0
		// env now, so make it 3.0 as minimum test:
		switch runtime.GOOS {
		case "linux":
			Expect(true).To(Equal(getKernelVersion(c) > 3.0))
		default:
			// no test
		}
	})
	It("Test getModelConfigMap", func() {
		configStr := "CONTAINER_COMPONENTS_ESTIMATOR=true\nCONTAINER_COMPONENTS_INIT_URL=https://raw.githubusercontent.com/sustainable-computing-io/kepler-model-server/main/tests/test_models/DynComponentPower/CgroupOnly/ScikitMixed/ScikitMixed.json\n"
		os.Setenv("MODEL_CONFIG", configStr)
		configValues := getModelConfigMap()
		modelItem := "CONTAINER_COMPONENTS"
		fmt.Printf("%s: %s", getModelConfigKey(modelItem, EstimatorEnabledKey), configValues[getModelConfigKey(modelItem, EstimatorEnabledKey)])
		useEstimatorSidecarStr := configValues[getModelConfigKey(modelItem, EstimatorEnabledKey)]
		Expect(useEstimatorSidecarStr).To(Equal("true"))
		initModelURL := configValues[getModelConfigKey(modelItem, InitModelURLKey)]
		Expect(initModelURL).NotTo(Equal(""))

	})
})
