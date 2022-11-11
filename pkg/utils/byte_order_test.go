package utils

import (
	"encoding/binary"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Byte order test", func() {
	It("Byte order test", func() {
		order := DetermineHostByteOrder()
		switch runtime.GOARCH {
		case "mips", "mips64", "ppc64", "s390x":
			Expect(order).To(Equal(binary.BigEndian))
		default:
			Expect(order).To(Equal(binary.LittleEndian))
		}
	})

	It("CreateTempFile", func() {
		filename, err := CreateTempFile("abc")
		Expect(err).NotTo(HaveOccurred())
		Expect(filename).ToNot(BeEmpty())
	})

	It("GetPathFromPID", func() {
		_, err := GetPathFromPID("", 1)
		Expect(err).To(HaveOccurred())
	})
})
