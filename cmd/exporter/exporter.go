/*
Copyright 2021.

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

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/bpf"
	"github.com/sustainable-computing-io/kepler/pkg/collector/stats"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/manager"
	"github.com/sustainable-computing-io/kepler/pkg/metrics"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/components"
	"github.com/sustainable-computing-io/kepler/pkg/sensors/platform"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/klog/v2"
)

var (
	// Version is the version of the exporter. Set by the linker flags in the Makefile.
	Version string

	// Revision is the Git commit that was compiled. Set by the linker flags in the Makefile.
	Revision string

	// Branch is the Git branch that was compiled. Set by the linker flags in the Makefile.
	Branch string

	// OS is the operating system the exporter was built for. Set by the linker flags in the Makefile.
	OS string

	// Arch is the architecture the exporter was built for. Set by the linker flags in the Makefile.
	Arch string
)

const (

	// to change these msg, you also need to update the e2e test
	finishingMsg = "Exiting..."
	startedMsg   = "Started Kepler in %s"
)

var (
	address                      = flag.String("address", "0.0.0.0:8888", "bind address")
	metricsPath                  = flag.String("metrics-path", "/metrics", "metrics path")
	enableGPU                    = flag.Bool("enable-gpu", false, "whether enable gpu (need to have libnvidia-ml installed)")
	enabledEBPFCgroupID          = flag.Bool("enable-cgroup-id", true, "whether enable eBPF to collect cgroup id (must have kernel version >= 4.18 and cGroup v2)")
	exposeHardwareCounterMetrics = flag.Bool("expose-hardware-counter-metrics", true, "whether expose hardware counter as prometheus metrics")
	enabledMSR                   = flag.Bool("enable-msr", false, "whether MSR is allowed to obtain energy data")
	kubeconfig                   = flag.String("kubeconfig", "", "absolute path to the kubeconfig file, if empty we use the in-cluster configuration")
	apiserverEnabled             = flag.Bool("apiserver", true, "if apiserver is disabled, we collect pod information from kubelet")
	redfishCredFilePath          = flag.String("redfish-cred-file-path", "", "path to the redfish credential file")
	exposeEstimatedIdlePower     = flag.Bool("expose-estimated-idle-power", false, "estimated idle power is meaningful only if Kepler is running on bare-metal or when there is only one virtual machine on the node")
)

func healthProbe(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`ok`))
	if err != nil {
		klog.Fatalf("%s", fmt.Sprintf("failed to write response: %v", err))
	}
}

func main() {
	start := time.Now()
	klog.InitFlags(nil)
	flag.Parse()

	klog.Infof("Kepler running on version: %s", Version)

	registry := metrics.GetRegistry()
	registry.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "kepler_exporter_build_info",
			Help: "A metric with a constant '1' value labeled by version, revision, branch, os and arch from which kepler_exporter was built.",
			ConstLabels: prometheus.Labels{
				"branch":   Branch,
				"revision": Revision,
				"version":  Version,
				"os":       OS,
				"arch":     Arch,
			},
		},
		func() float64 { return 1 },
	))

	config.SetEnabledEBPFCgroupID(*enabledEBPFCgroupID)
	config.SetEnabledHardwareCounterMetrics(*exposeHardwareCounterMetrics)
	config.SetEnabledGPU(*enableGPU)
	config.SetEnabledMSR(*enabledMSR)
	config.SetEnabledIdlePower(*exposeEstimatedIdlePower || components.IsSystemCollectionSupported())

	config.SetKubeConfig(*kubeconfig)
	config.SetEnableAPIServer(*apiserverEnabled)

	// set redfish credential file path
	if *redfishCredFilePath != "" {
		config.SetRedfishCredFilePath(*redfishCredFilePath)
	}

	config.LogConfigs()

	components.InitPowerImpl()
	defer components.StopPower()
	platform.InitPowerImpl()
	defer platform.StopPower()

	stats.InitAvailableParamAndMetrics()

	if config.EnabledGPU {
		r := accelerator.GetRegistry()
		if a, err := accelerator.New(accelerator.GPU, true); err == nil {
			r.MustRegister(a) // Register the accelerator with the registry
		} else {
			klog.Errorf("failed to init GPU accelerators: %v", err)
		}
		defer accelerator.Shutdown()
	}

	bpfExporter, err := bpf.NewExporter()
	if err != nil {
		klog.Fatalf("failed to create eBPF exporter: %v", err)
	}
	defer bpfExporter.Detach()

	m := manager.New(bpfExporter)

	// starting a CollectorManager instance to collect data and report metrics
	if startErr := m.Start(); startErr != nil {
		klog.Infof("%s", fmt.Sprintf("failed to start : %v", startErr))
	}
	metricPathConfig := config.GetMetricPath(*metricsPath)
	bindAddressConfig := config.GetBindAddress(*address)

	handler := http.ServeMux{}
	reg := m.PrometheusCollector.RegisterMetrics()
	handler.Handle(metricPathConfig, promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{
			Registry: reg,
		},
	))
	handler.HandleFunc("/healthz", healthProbe)
	handler.HandleFunc("/", rootHandler(metricPathConfig))
	handler.HandleFunc("/debug/pprof/", http.DefaultServeMux.ServeHTTP)
	srv := &http.Server{
		Addr:    bindAddressConfig,
		Handler: &handler,
	}

	klog.Infof("starting to listen on %s", bindAddressConfig)
	errChan := make(chan error)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()
	klog.Infof(startedMsg, time.Since(start))
	klog.Flush() // force flush to parse the start msg in the e2e test

	// Wait for an exit signal

	ctx := context.Background()
	select {
	case err := <-errChan:
		klog.Fatalf("%s", fmt.Sprintf("failed to listen and serve: %v", err))
	case <-signalChan:
		klog.Infof("Received shutdown signal")
		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Second))
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			klog.Fatalf("%s", fmt.Sprintf("failed to shutdown gracefully: %v", err))
		}
	}
	wg.Wait()
	klog.Infoln(finishingMsg)
	klog.FlushAndExit(klog.ExitFlushTimeout, 0)
}

func rootHandler(metricPathConfig string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`<html>
					<head><title>Energy Stats Exporter</title></head>
					<body>
					<h1>Energy Stats Exporter</h1>
					<p><a href="` + metricPathConfig + `">Metrics</a></p>
					</body>
					</html>`)); err != nil {
			klog.Errorf("%s", fmt.Sprintf("failed to write http response: %v", err))
		}
	}
}
