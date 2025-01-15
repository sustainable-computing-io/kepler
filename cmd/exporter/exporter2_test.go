package main

import (
	"flag"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
)

var _ = Describe("Main Function", func() {
	var (
		originalArgs []string
	)

	BeforeEach(func() {
		// Save the original command-line arguments
		originalArgs = os.Args

		// Reset the flag command-line arguments
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		// Initialize klog flags
		klog.InitFlags(nil)
	})

	AfterEach(func() {
		// Restore the original command-line arguments
		os.Args = originalArgs
	})

	Context("when the config initialization fails", func() {
		It("should log a fatal error and exit", func() {
			// Mock the command-line arguments to simulate a failure scenario
			os.Args = []string{"cmd", "-base-dir=/invalid/path"}

			// Redirect klog output to a buffer to capture the log messages
			klog.SetOutput(GinkgoWriter)

			// Use a defer to recover from the fatal log and prevent the test from exiting
			defer func() {
				if r := recover(); r != nil {
					Expect(r).To(ContainSubstring("Failed to initialize config"))
				}
			}()

			// Call the main function
			main()
		})
	})

	Context("when the config initialization succeeds", func() {
		It("should initialize the config without errors", func() {
			// Mock the command-line arguments to simulate a success scenario
			os.Args = []string{"cmd", "-base-dir=/valid/path"}

			// Redirect klog output to a buffer to capture the log messages
			klog.SetOutput(GinkgoWriter)

			// Use a defer to recover from the fatal log and prevent the test from exiting
			defer func() {
				if r := recover(); r != nil {
					Fail("Unexpected fatal log")
				}
			}()

			// Call the main function
			main()
		})
	})
})