//go:build linux && libbpf
// +build linux,libbpf

package bpftest

import (
	"testing"

	"github.com/cilium/ebpf/rlimit"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/bpf"
)

func TestBpf(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bpf Suite")
}

func checkDataCollected(processesData []bpf.ProcessMetrics) {
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
	It("should increment the page cache hit counter", func() {
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadTest()
		Expect(err).NotTo(HaveOccurred())

		err = specs.RewriteConstants(map[string]interface{}{
			"TEST": int32(1),
		})
		Expect(err).NotTo(HaveOccurred())

		var obj testObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		key := uint32(0)

		err = obj.Processes.Put(key, testProcessMetricsT{
			CgroupId:       0,
			Pid:            0,
			ProcessRunTime: 0,
			CpuCycles:      0,
			CpuInstr:       0,
			CacheMiss:      0,
			PageCacheHit:   0,
			VecNr:          [10]uint16{},
			Comm:           [16]int8{},
		})
		Expect(err).NotTo(HaveOccurred())

		// XDP Programs must have at least 14 bytes of data in the context
		in := make([]byte, 0, 14)
		out, _, err := obj.TestKeplerWritePageTrace.Test(in)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(Equal(uint32(0)))

		// Read the page cache hit counter
		var res testProcessMetricsT
		err = obj.Processes.Lookup(key, &res)
		Expect(err).NotTo(HaveOccurred())

		Expect(res.PageCacheHit).To(BeNumerically("==", uint64(1)))

		err = obj.Processes.Delete(key)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should register a new process if one doesn't exist", func() {
		// Remove resource limits for kernels <5.11.
		err := rlimit.RemoveMemlock()
		Expect(err).NotTo(HaveOccurred())

		// Load eBPF Specs
		specs, err := loadTest()
		Expect(err).NotTo(HaveOccurred())

		err = specs.RewriteConstants(map[string]interface{}{
			"TEST": int32(1),
		})
		Expect(err).NotTo(HaveOccurred())

		var obj testObjects
		// Load eBPF objects
		err = specs.LoadAndAssign(&obj, nil)
		Expect(err).NotTo(HaveOccurred())

		// XDP Programs must have at least 14 bytes of data in the context
		in := make([]byte, 0, 14)
		out, _, err := obj.TestRegisterNewProcessIfNotExist.Test(in)
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(BeNumerically("==", uint32(0)))

		// Read the page cache hit counter
		var res testProcessMetricsT
		key := uint32(42) // Kernel TGID
		err = obj.Processes.Lookup(key, &res)
		Expect(err).NotTo(HaveOccurred())

		Expect(res.Pid).To(BeNumerically("==", uint64(42)))

		err = obj.Processes.Delete(key)
		Expect(err).NotTo(HaveOccurred())
	})
})
