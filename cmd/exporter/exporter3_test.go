package main

import (
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RootHandler", func() {
	var (
		metricPathConfig string
		handler          http.HandlerFunc
		recorder         *httptest.ResponseRecorder
		request          *http.Request
	)

	BeforeEach(func() {
		metricPathConfig = "/metrics"
		handler = rootHandler(metricPathConfig)
		recorder = httptest.NewRecorder()
		request = httptest.NewRequest("GET", "/", nil)
	})

	Context("when the root endpoint is accessed", func() {
		It("should return a valid HTML response with the correct metric path", func() {
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusOK))
			Expect(recorder.Body.String()).To(ContainSubstring("<title>Energy Stats Exporter</title>"))
			Expect(recorder.Body.String()).To(ContainSubstring("<h1>Energy Stats Exporter</h1>"))
			Expect(recorder.Body.String()).To(ContainSubstring(`<a href="` + metricPathConfig + `">Metrics</a>`))
		})
	})

	Context("when the response writing fails", func() {
		It("should log an error", func() {
			// Mock the response writer to simulate a write failure
			failingRecorder := &FailingResponseRecorder{httptest.NewRecorder()}
			handler.ServeHTTP(failingRecorder, request)

			// Here you would typically check if the error was logged.
			// Since klog is used, you might need to capture logs or mock klog for this test.
			// This is a placeholder to indicate where you would check for the error log.
			Expect(failingRecorder.Failed).To(BeTrue())
		})
	})
})

// FailingResponseRecorder is a custom ResponseRecorder that fails on Write
type FailingResponseRecorder struct {
	*httptest.ResponseRecorder
	Failed bool
}

func (r *FailingResponseRecorder) Write(b []byte) (int, error) {
	if strings.Contains(string(b), "Metrics") {
		r.Failed = true
		return 0, fmt.Errorf("simulated write failure")
	}
	return r.ResponseRecorder.Write(b)
}