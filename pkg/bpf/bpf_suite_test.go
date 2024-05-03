package bpf

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBpf(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bpf Suite")
}

func checkDataCollected(processesData []ProcessMetrics) {
	Expect(len(processesData)).To(BeNumerically(">", 0))
	for _, p := range processesData {
		Expect(p.Pid).To(BeNumerically(">=", 0))
		Expect(p.Comm).NotTo(BeEmpty())
		Expect(p.CpuCycles).To(BeNumerically(">=", uint64(0)))
		Expect(p.CpuInstr).To(BeNumerically(">=", uint64(0)))
		Expect(p.CacheMiss).To(BeNumerically(">=", uint64(0)))
		Expect(p.CgroupId).To(BeNumerically(">=", uint64(0)))
	}
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
