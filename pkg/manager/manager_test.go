//go:build !bcc
// +build !bcc

package manager

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var _ = Describe("Manager", func() {

	It("Should work properly", func() {
		CollectorManager := New()
		Expect(float64(config.SamplePeriodSec)).To(Equal(CollectorManager.PrometheusCollector.SamplePeriodSec))
		err := CollectorManager.Start()
		// for no bcc tag in CI
		Expect(err).To(HaveOccurred())
	})

})
