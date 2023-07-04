package attacher

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAttacher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Attacher Suite")
}

func checkDataCollected(processesData []ProcessBPFMetrics, cpuFreqData map[int32]uint64) {
	// len > 0
	Expect(len(processesData)).To(BeNumerically(">", 0))
	Expect(len(cpuFreqData)).To(BeNumerically(">", 0))
	// freq must have a value
	Expect(cpuFreqData[0]).To(BeNumerically(">", 0))
}

var _ = Describe("BPF attacher test", func() {
	It("should attach bpf module", func() {
		defer Detach()
		if BccBuilt || LibbpfBuilt {
			_, err := Attach()
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(time.Second * 1) // wait for some data
			processesData, err := CollectProcesses()
			Expect(err).NotTo(HaveOccurred())
			cpuFreqData, err := CollectCPUFreq()
			Expect(err).NotTo(HaveOccurred())
			checkDataCollected(processesData, cpuFreqData)
			Detach()
		}
	})
})
