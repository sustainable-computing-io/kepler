//go:build !darwin
// +build !darwin

package manager

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/bpf"
)

var _ = Describe("Manager", func() {

	It("Should work properly", func() {
		bpfExporter := bpf.NewMockExporter(bpf.DefaultSupportedMetrics())
		CollectorManager := New(bpfExporter)
		err := CollectorManager.Start()
		Expect(err).NotTo(HaveOccurred())
	})

})
