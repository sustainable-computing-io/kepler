/*
Copyright 2023.

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

package libvirt

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	cGroupFileName = "cgroup"
	cGroupContent  = "0::/machine.slice/machine-qemu\x2d6\x2dcirros.scope/libvirt/emulator"
)

var _ = Describe("Test LibVirt", func() {
	var (
		mockProcDir = ""
	)

	BeforeEach(func() {
		mockProcDir = createTempDir()
	})

	AfterEach(func() {
		removeTempDir(mockProcDir)
	})

	It("Test GetCurrentVMPID", func() {
		pid := uint64(13)
		fmt.Fprintln(GinkgoWriter, "mockProcDir", mockProcDir)
		fileName := createMockProcDir(mockProcDir, pid)
		vmID, err := getVMID(pid, fileName)
		Expect(err).NotTo(HaveOccurred())
		Expect(vmID).Should(Equal("machine-qemu-6-cirros"))
	})
})

// helper function to create a temporary directory
func createTempDir() string {
	tmpDir, err := os.MkdirTemp("", "ginkgo-temp")
	Expect(err).NotTo(HaveOccurred())
	return tmpDir
}

// helper function to remove the temporary directory
func removeTempDir(dir string) {
	err := os.RemoveAll(dir)
	Expect(err).NotTo(HaveOccurred())
}

// helper function to create a temporary process directory
func createMockProcDir(directory string, pid uint64) string {
	procDir := fmt.Sprintf("/proc/%d/", pid)
	fullPathProcDir := filepath.Join(directory, procDir)
	err := os.MkdirAll(fullPathProcDir, 0755)
	Expect(err).NotTo(HaveOccurred())

	cgroupFile := filepath.Join(fullPathProcDir, cGroupFileName)
	createMockProcCGroupDir(cgroupFile)
	return cgroupFile
}

// helper function to write a temporary cgroup info
func createMockProcCGroupDir(filename string) {
	err := os.WriteFile(filename, []byte(cGroupContent), 0644)
	Expect(err).NotTo(HaveOccurred())
}
