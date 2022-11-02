package e2e_test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const (
	timeout         = 60
	poolingInterval = 5
)

var _ = Describe("healthz check should pass", func() {
	Context("start server with health check", func() {
		It("should work properly", func() {
			address, ok := os.LookupEnv("kepler_address")
			if !ok {
				address = "localhost:8888"
				cmd := exec.Command(keplerBin)
				keplerSession, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() string {
					sdtErr := string(keplerSession.Err.Contents())
					fmt.Println("keplerSession sdtErr", sdtErr)
					return sdtErr
				}, timeout, poolingInterval).Should(Or(ContainSubstring("Started Kepler"), ContainSubstring("exiting...")))
			}
			resp, err := http.Get("http://" + address + "/healthz")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})
})
