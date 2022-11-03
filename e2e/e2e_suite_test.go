package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	tmpDir, keplerBin, address string
	keplerSession              *gexec.Session
	err                        error
	ok                         bool
)

func TestE2eTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2eTest Suite")
}

var _ = BeforeSuite(func() {
	address, ok = os.LookupEnv("kepler_address")
	if !ok {
		tmpDir, err = os.MkdirTemp("", "test-kepler")
		Expect(err).NotTo(HaveOccurred())
		keplerBin, err = gexec.Build("../cmd/exporter.go")
		Expect(err).NotTo(HaveOccurred())
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
})

var _ = AfterSuite(func() {
	if keplerSession != nil {
		keplerSession.Kill()
	}
	os.RemoveAll(tmpDir)
	os.RemoveAll(keplerBin)
})
