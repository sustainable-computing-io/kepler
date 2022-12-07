package bpfassets

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBpfassets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bpfassets Suite")
}

var _ = Describe("BPF asset generation", func() {
	It("should generate BPF assets", func() {
		program := Program
		_, err := Asset(program)
		Expect(err).NotTo(HaveOccurred())
	})
})
