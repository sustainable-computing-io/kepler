package e2e_test

import (
	"net/http"
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
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
				time.Sleep(5 * time.Second) // wait for server start up
			}
			resp, err := http.Get("http://" + address + "/healthz")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})
})
