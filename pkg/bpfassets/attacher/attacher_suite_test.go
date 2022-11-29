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
