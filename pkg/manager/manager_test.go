//go:build !bcc
// +build !bcc

package manager

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {

	It("Should work properly", func() {
		CollectorManager := New()
		err := CollectorManager.Start()
		// for no bcc tag in CI
		Expect(err).To(HaveOccurred())
	})

})
