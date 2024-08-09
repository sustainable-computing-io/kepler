package comm

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCollector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Comm Resolver Suite")
}

func resolveFromCmdline(pid int) (string, error) {
	return readCommandFromProcFsCmdline([]byte("cmdline\x00is\x00a\x000test")), nil
}

func resolveFromComm(pid int) (string, error) {
	return readCommandFromProcFsComm([]byte("comm")), nil
}

func resolveNotExist(pid int) (string, error) {
	return unknownComm, os.ErrNotExist
}

var _ = Describe("CommResolver", func() {
	Describe("ResolveComm", func() {
		Context("when the process ID exists", func() {
			It("should return the resolved command name from cmdline", func() {
				resolver := NewTestCommResolver(resolveFromCmdline)
				resolvedComm, err := resolver.ResolveComm(1234)
				Expect(err).ToNot(HaveOccurred())
				Expect(resolvedComm).To(Equal("cmdline"))
				// Verify that the resolved command name is cached
				Expect(resolver.cacheExist).To(HaveKey(1234))
				Expect(resolver.cacheExist[1234]).To(Equal("cmdline"))
			})

			It("should return the resolved command name from comm", func() {
				resolver := NewTestCommResolver(resolveFromComm)
				resolvedComm, err := resolver.ResolveComm(1234)
				Expect(err).ToNot(HaveOccurred())
				Expect(resolvedComm).To(Equal("[comm]"))
				// Verify that the resolved command name is cached
				Expect(resolver.cacheExist).To(HaveKey(1234))
				Expect(resolver.cacheExist[1234]).To(Equal("[comm]"))
			})
		})

		Context("when the process ID does not exist", func() {
			It("should return an error", func() {
				resolver := NewTestCommResolver(resolveNotExist)
				resolvedComm, err := resolver.ResolveComm(54321)
				Expect(err).To(HaveOccurred())
				Expect(resolvedComm)
				// Verify that the process ID is cached as non-existent
				Expect(resolver.cacheNotExist).To(HaveKey(54321))
			})
		})
	})

	Describe("Clear", func() {
		It("should clear the cache for freed process IDs", func() {
			freed := []int{123, 456, 789}
			resolver := NewTestCommResolver(resolveFromCmdline)

			// Add some entries to the cache
			for _, pid := range freed {
				_, err := resolver.ResolveComm(pid)
				Expect(err).ToNot(HaveOccurred())
			}

			// Clear the cache
			resolver.Clear(freed)

			// Verify that the cache is empty for the freed process IDs
			Expect(resolver.cacheExist).To(HaveLen(0))
			Expect(resolver.cacheNotExist).To(HaveLen(0))
		})
	})
})
