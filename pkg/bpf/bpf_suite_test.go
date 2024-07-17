package bpf

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBpf(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bpf Suite")
}
