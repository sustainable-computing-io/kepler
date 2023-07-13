//go:build bcc
// +build bcc

package attacher

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAttacher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Attacher Suite")
}

var _ = Describe("BPF attacher test", func() {
	It("should attach bpf module", func() {
		m, err := AttachBPFAssets()
		// if build with -tags=bcc, the bpf module will be attached successfully
		Expect(err).NotTo(HaveOccurred())
		DetachBPFModules(m)
	})
})
