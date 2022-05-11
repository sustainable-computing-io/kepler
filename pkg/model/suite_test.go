package model

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Model Suite")
}

func checkPathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

var _ = BeforeSuite(func() {
	// check model path exists
	Expect(checkPathExists(MODEL_DB_PATH)).To(Equal(true))
	Expect(checkPathExists(MODEL_PATH)).To(Equal(true))
	Expect(checkPathExists(METADATA_PATH)).To(Equal(true))
})

var _ = AfterSuite(func() {
})
