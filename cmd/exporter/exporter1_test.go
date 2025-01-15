package main

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HealthProbe", func() {
	var (
		w *httptest.ResponseRecorder
		req *http.Request
	)

	BeforeEach(func() {
		// Initialize the response recorder and request before each test
		w = httptest.NewRecorder()
		req = httptest.NewRequest("GET", "/health", nil)
	})

	Context("when the health probe is called", func() {
		It("should return HTTP status OK", func() {
			healthProbe(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return 'ok' in the response body", func() {
			healthProbe(w, req)

			Expect(w.Body.String()).To(Equal("ok"))
		})
	})
})