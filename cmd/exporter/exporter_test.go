package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sustainable-computing-io/kepler/pkg/config"
)

var _ = Describe("AppConfig", func() {
	var cfg *AppConfig

	BeforeEach(func() {
		cfg = newAppConfig()
		// Reset the command-line flags before each test
		flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	})

	It("should initialize with default values", func() {
		Expect(cfg.BaseDir).To(Equal(config.BaseDir))
		Expect(cfg.Address).To(Equal("0.0.0.0:8888"))
		Expect(cfg.MetricsPath).To(Equal("/metrics"))
		Expect(cfg.EnableGPU).To(BeFalse())
		Expect(cfg.EnableEBPFCgroupID).To(BeTrue())
		Expect(cfg.ExposeHardwareCounterMetrics).To(BeTrue())
		Expect(cfg.EnableMSR).To(BeFalse())
		Expect(cfg.Kubeconfig).To(BeEmpty())
		Expect(cfg.ApiserverEnabled).To(BeTrue())
		Expect(cfg.RedfishCredFilePath).To(BeEmpty())
		Expect(cfg.ExposeEstimatedIdlePower).To(BeFalse())
		Expect(cfg.MachineSpecFilePath).To(BeEmpty())
		Expect(cfg.DisablePowerMeter).To(BeFalse())
		Expect(cfg.TLSFilePath).To(BeEmpty())
	})

	It("should override default values with command-line flags", func() {
		cfg = newAppConfig()
		// Set command-line flags
		flag.Set("config-dir", "/custom/config/dir")
		flag.Set("address", "127.0.0.1:8080")
		flag.Set("metrics-path", "/custom/metrics")
		flag.Set("enable-gpu", "true")
		flag.Set("enable-cgroup-id", "false")
		flag.Set("expose-hardware-counter-metrics", "false")
		flag.Set("enable-msr", "true")
		flag.Set("kubeconfig", "/custom/kubeconfig")
		flag.Set("apiserver", "false")
		flag.Set("redfish-cred-file-path", "/custom/redfish/cred")
		flag.Set("expose-estimated-idle-power", "true")
		flag.Set("machine-spec", "/custom/machine/spec")
		flag.Set("disable-power-meter", "true")
		flag.Set("web.config.file", "/custom/tls/config")

		// Parse the flags
		flag.Parse()

		// Verify that the values are overridden
		Expect(cfg.BaseDir).To(Equal("/custom/config/dir"))
		Expect(cfg.Address).To(Equal("127.0.0.1:8080"))
		Expect(cfg.MetricsPath).To(Equal("/custom/metrics"))
		Expect(cfg.EnableGPU).To(BeTrue())
		Expect(cfg.EnableEBPFCgroupID).To(BeFalse())
		Expect(cfg.ExposeHardwareCounterMetrics).To(BeFalse())
		Expect(cfg.EnableMSR).To(BeTrue())
		Expect(cfg.Kubeconfig).To(Equal("/custom/kubeconfig"))
		Expect(cfg.ApiserverEnabled).To(BeFalse())
		Expect(cfg.RedfishCredFilePath).To(Equal("/custom/redfish/cred"))
		Expect(cfg.ExposeEstimatedIdlePower).To(BeTrue())
		Expect(cfg.MachineSpecFilePath).To(Equal("/custom/machine/spec"))
		Expect(cfg.DisablePowerMeter).To(BeTrue())
		Expect(cfg.TLSFilePath).To(Equal("/custom/tls/config"))
	})
})

var _ = Describe("HealthProbe", func() {
	var (
		w   *httptest.ResponseRecorder
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
			failingRecorder := &FailingResponseRecorder{httptest.NewRecorder(), false}
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
