//go:build linux && libbpf
// +build linux,libbpf

package bpf

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBpf(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Attacher Suite")
}

func checkDataCollected(processesData []ProcessBPFMetrics) {
	// len > 0
	Expect(len(processesData)).To(BeNumerically(">", 0))
	Expect(processesData[0].PID).To(BeNumerically(">", 0))
	Expect(processesData[0].Command).NotTo(BeEmpty())
	Expect(processesData[0].CPUCycles).To(BeNumerically(">=", 0))
	Expect(processesData[0].CPUInstr).To(BeNumerically(">=", 0))
	Expect(processesData[0].CacheMisses).To(BeNumerically(">=", 0))
	Expect(processesData[0].ThreadPID).To(BeNumerically(">", 0))
	Expect(processesData[0].TaskClockTime).To(BeNumerically(">=", 0))
	Expect(processesData[0].CGroupID).To(BeNumerically(">", 0))
}

var _ = Describe("BPF Exporter test", func() {
	It("should attach bpf module", func() {
		a, err := NewExporter()
		Expect(err).NotTo(HaveOccurred())
		defer a.Detach()
		time.Sleep(time.Second * 1) // wait for some data
		processesData, err := a.CollectProcesses()
		Expect(err).NotTo(HaveOccurred())
		checkDataCollected(processesData)
	})
})
