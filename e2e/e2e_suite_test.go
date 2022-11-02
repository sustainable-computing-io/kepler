package e2e_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	tmpDir, keplerBin string
	keplerSession     *gexec.Session
	err               error
)

func TestE2eTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2eTest Suite")
}

var _ = BeforeSuite(func() {
	tmpDir, err = os.MkdirTemp("", "test-kepler")
	Expect(err).NotTo(HaveOccurred())
	keplerBin, err = gexec.Build("../cmd/exporter.go")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterEach(func() {
	if keplerSession != nil {
		keplerSession.Kill()
	}
})

var _ = AfterSuite(func() {
	os.RemoveAll(tmpDir)
	os.RemoveAll(keplerBin)
})
