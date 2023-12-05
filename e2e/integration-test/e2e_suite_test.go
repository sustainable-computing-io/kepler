/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integrationtest

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const (
	DefaultAddress = "localhost:9102"
	KeplerEnvVar   = "kepler_address"
)

var (
	tmpDir, keplerBin, address string
	keplerSession              *gexec.Session
)

func TestE2eTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2eTest Suite")
}

var _ = BeforeSuite(func() {
	var ok bool
	address, ok = os.LookupEnv(KeplerEnvVar)
	if !ok {
		var err error
		tmpDir, err = os.MkdirTemp("", "test-kepler")
		Expect(err).NotTo(HaveOccurred())
		keplerBin, err = gexec.Build("../cmd/exporter.go")
		Expect(err).NotTo(HaveOccurred())
		address = DefaultAddress
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
