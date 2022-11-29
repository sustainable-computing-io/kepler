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
