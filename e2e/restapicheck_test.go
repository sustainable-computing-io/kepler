package e2e_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	timeout         = 60
	poolingInterval = 5
)

var _ = Describe("request check should pass", func() {

	DescribeTable("Test with endpoints with requests", func(path string) {
		resp, err := http.Get("http://" + address + "/" + path)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	},
		Entry("default endpoint", ""),
		Entry("default healthz", "healthz"),
		Entry("default metrics", "metrics"),
	)
})
