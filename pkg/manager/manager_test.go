//go:build !darwin
// +build !darwin

package manager

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	bpfAttacher "github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
)

var _ = Describe("Manager", func() {

	It("Should work properly", func() {

		attacher := bpfAttacher.NewMockAttacher(true)
		CollectorManager := New(attacher)
		err := CollectorManager.Start()
		Expect(err).NotTo(HaveOccurred())
	})

})
