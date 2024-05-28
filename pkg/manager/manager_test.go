//go:build !darwin
// +build !darwin

package manager

import (
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/bpf"
)

var _ = Describe("Manager", func() {

	It("Should work properly", func() {
		bpfExporter := bpf.NewMockExporter(bpf.DefaultSupportedMetrics())
		CollectorManager := New(bpfExporter)
		stopChan := make(chan struct{})
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := CollectorManager.Start(stopChan)
			Expect(err).NotTo(HaveOccurred())
		}()
		close(stopChan)
		wg.Wait()
	})

})
