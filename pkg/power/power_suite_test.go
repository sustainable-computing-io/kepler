package power

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPower(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Power Suite")
}
