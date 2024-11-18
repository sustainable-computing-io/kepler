package manager

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var _ = Describe("Manager", func() {

	It("Should work properly", func() {
		_, err := config.Initialize(".")
		Expect(err).NotTo(HaveOccurred())
		bpfExporter := bpf.NewMockExporter(bpf.DefaultSupportedMetrics())
		CollectorManager := New(bpfExporter)
		err = CollectorManager.Start()
		Expect(err).NotTo(HaveOccurred())
	})

})
