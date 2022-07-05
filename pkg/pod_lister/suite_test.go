package pod_lister

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
)

func TestPodLoader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pod Lister Suite")
}

var _ = BeforeSuite(func() {
	os.Setenv("KUBECONFIG_FILE", "/root/.kube/config")
	InitKeeper()
	go Keeper.Run()
})

var _ = AfterSuite(func() {
	os.Unsetenv("KUBECONFIG_FILE")
	Destroy()
})

