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
	"encoding/json"
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

func (spec *MachineSpec) saveToFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	return encoder.Encode(spec)
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
			Expect(true).To(Equal(getKernelVersion(&realSystem{}) > 3.0))
		default:
			// no test
		}
	})
	It("Test machine spec generation and read", func() {
		tmpPath := "./test_spec"
		// generate spec
		spec := GenerateSpec()
		Expect(spec).NotTo(BeNil())
		err := spec.saveToFile(tmpPath)
		Expect(err).To(BeNil())
		readSpec, err := readMachineSpec(tmpPath)
		Expect(err).To(BeNil())
		Expect(*spec).To(BeEquivalentTo(*readSpec))
		err = os.Remove(tmpPath)
		Expect(err).To(BeNil())
	})
	It("test init by default", func() {
		Config, err := Initialize(".")
		Expect(err).NotTo(HaveOccurred())
		Expect(Config.Kepler).NotTo(BeNil())
		Expect(Config.KernelVersion).To(Equal(float32(0)))
		Expect(IsExposeProcessStatsEnabled()).To(BeFalse())
		Expect(IsExposeContainerStatsEnabled()).To(BeTrue())
		Expect(IsExposeVMStatsEnabled()).To(BeTrue())
		Expect(IsExposeBPFMetricsEnabled()).To(BeTrue())
		Expect(IsExposeComponentPowerEnabled()).To(BeTrue())
		Expect(ExposeIRQCounterMetrics()).To(BeTrue())
		Expect(GetBPFSampleRate()).To(Equal(0))

	})
	It("test init by set func and Is Enable functions", func() {
		Config, err := Initialize(".")
		Expect(err).NotTo(HaveOccurred())
		// test set and is enable functions.
		SetEnabledGPU(true)
		Expect(Config.Kepler.EnabledGPU).To(BeTrue())
		Expect(IsGPUEnabled()).To(BeTrue())
		SetEnabledGPU(false)
		Expect(Config.Kepler.EnabledGPU).To(BeFalse())
		Expect(IsGPUEnabled()).To(BeFalse())

		SetEnabledMSR(true)
		Expect(Config.Kepler.EnabledMSR).To(BeTrue())
		Expect(IsEnabledMSR()).To(BeTrue())
		SetEnabledMSR(false)
		Expect(Config.Kepler.EnabledMSR).To(BeFalse())
		Expect(IsEnabledMSR()).To(BeFalse())

		SetEnableAPIServer(true)
		Expect(Config.Kepler.EnableAPIServer).To(BeTrue())
		Expect(IsAPIServerEnabled()).To(BeTrue())
		SetEnableAPIServer(false)
		Expect(Config.Kepler.EnableAPIServer).To(BeFalse())
		Expect(IsAPIServerEnabled()).To(BeFalse())

		SetMachineSpecFilePath("dummy")
		Expect(Config.Kepler.MachineSpecFilePath).To(Equal("dummy"))

		SetEnabledIdlePower(true)
		Expect(Config.Kepler.ExposeIdlePowerMetrics).To(BeTrue())
		Expect(IsIdlePowerEnabled()).To(BeTrue())
		SetEnabledIdlePower(false)
		Expect(Config.Kepler.ExposeIdlePowerMetrics).To(BeFalse())
		Expect(IsIdlePowerEnabled()).To(BeFalse())

		SetEnabledHardwareCounterMetrics(true)
		Expect(Config.Kepler.ExposeHardwareCounterMetrics).To(BeTrue())
		Expect(ExposeHardwareCounterMetrics()).To(BeTrue())
		SetEnabledHardwareCounterMetrics(false)
		Expect(Config.Kepler.ExposeHardwareCounterMetrics).To(BeFalse())
		Expect(ExposeHardwareCounterMetrics()).To(BeFalse())
	})
})
