package podlister

import (
	"encoding/binary"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Byte order test", func() {
	It("Byte order test", func() {
		order := determineHostByteOrder()
		switch runtime.GOARCH {
		case "mips", "mips64", "ppc64", "s390x":
			Expect(order).To(Equal(binary.BigEndian))
		default:
			Expect(order).To(Equal(binary.LittleEndian))
		}
	})
})
