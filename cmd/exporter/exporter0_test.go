package your_package_name

import (
    "flag"
    "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("AppConfig", func() {
    var cfg *AppConfig

    ginkgo.BeforeEach(func() {
        // Reset the command-line flags before each test
        flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
        cfg = newAppConfig()
    })

    ginkgo.It("should initialize with default values", func() {
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

    ginkgo.It("should override default values with command-line flags", func() {
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

        // Reinitialize the config with the new flag values
        cfg = newAppConfig()

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