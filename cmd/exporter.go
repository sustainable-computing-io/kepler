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
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/bpfassets/attacher"
	collector_metric "github.com/sustainable-computing-io/kepler/pkg/collector/metric"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/manager"
	"github.com/sustainable-computing-io/kepler/pkg/model"
	"github.com/sustainable-computing-io/kepler/pkg/power/accelerator"
	"github.com/sustainable-computing-io/kepler/pkg/power/components"
	"github.com/sustainable-computing-io/kepler/pkg/power/platform"
	kversion "github.com/sustainable-computing-io/kepler/pkg/version"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"

	"k8s.io/klog/v2"
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
	cpuProfile                   = flag.String("cpuprofile", "", "dump cpu profile to a file")
	memProfile                   = flag.String("memprofile", "", "dump mem profile to a file")
	profileDuration              = flag.Int("profile-duration", 60, "duration in seconds")
	enabledMSR                   = flag.Bool("enable-msr", false, "whether MSR is allowed to obtain energy data")
	enabledBPFBatchDelete        = flag.Bool("enable-bpf-batch-del", true, "bpf map batch deletion can be enabled for backported kernels older than 5.6")
	kubeconfig                   = flag.String("kubeconfig", "", "absolute path to the kubeconfig file, if empty we use the in-cluster configuration")
	apiserverEnabled             = flag.Bool("apiserver", true, "if apiserver is disabled, we collect pod information from kubelet")
	kernelSourceDirPath          = flag.String("kernel-source-dir", "", "path to the kernel source directory")
	redfishCredFilePath          = flag.String("redfish-cred-file-path", "", "path to the redfish credential file")
)

func healthProbe(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`ok`))
	if err != nil {
		klog.Fatalf("%s", fmt.Sprintf("failed to write response: %v", err))
	}
}

func finalizing() {
	stack := "exit stack: \n" + string(debug.Stack())
	klog.Infof(stack)
	exitCode := 10
	klog.Infoln(finishingMsg)
	klog.FlushAndExit(klog.ExitFlushTimeout, exitCode)
}

func startProfiling(cpuProfile, memProfile string) {
	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			klog.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			klog.Fatal("could not start CPU profile: ", err)
		}
		klog.Infof("Started CPU profiling")
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			defer f.Close()
			exit := false
			select {
			case <-sigs:
				exit = true
				break
			case <-time.After(time.Duration(*profileDuration) * time.Second):
				break
			}
			pprof.StopCPUProfile()
			klog.Infof("Stopped CPU profiling")
			if exit {
				time.Sleep(time.Second) // sleep 1s to make sure that the mem profile could finish
				os.Exit(0)
			}
		}()
	}
	if memProfile != "" {
		f, err := os.Create(memProfile)
		if err != nil {
			klog.Fatal("could not create memory profile: ", err)
		}
		klog.Infof("Started Memory profiling")
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			defer f.Close()
			exit := false
			select {
			case <-sigs:
				exit = true
				break
			case <-time.After(time.Duration(*profileDuration) * time.Second):
				break
			}
			runtime.GC() // get up-to-date statistics
			if err := pprof.WriteHeapProfile(f); err != nil {
				klog.Fatal("could not write memory profile: ", err)
			}
			klog.Infof("Stopped Memory profiling")
			if exit {
				time.Sleep(time.Second) // sleep 1s to make sure that the cpu profile could finish
				os.Exit(0)
			}
		}()
	}
}

func main() {
	start := time.Now()
	defer finalizing()
	klog.InitFlags(nil)
	flag.Parse()

	klog.Infof("Kepler running on version: %s", kversion.Version)

	config.SetEnabledEBPFCgroupID(*enabledEBPFCgroupID)
	config.SetEnabledHardwareCounterMetrics(*exposeHardwareCounterMetrics)
	config.SetEnabledGPU(*enableGPU)
	config.EnabledMSR = *enabledMSR

	config.SetKubeConfig(*kubeconfig)
	config.SetEnableAPIServer(*apiserverEnabled)
	// try setting kernel source only for bcc build
	if attacher.BccBuilt {
		if kernelSourceDirPath != nil && len(*kernelSourceDirPath) > 0 {
			if err := config.SetKernelSourceDir(*kernelSourceDirPath); err != nil {
				klog.Warningf("failed to set kernel source dir to %q: %v", *kernelSourceDirPath, err)
			}
		}
	}

	// the ebpf batch deletion operation was introduced in linux kernel 5.6, which provides better performance to delete keys.
	// but the user can enable it if the kernel has backported this functionality.
	config.EnabledBPFBatchDelete = *enabledBPFBatchDelete
	if config.KernelVersion >= 5.6 {
		config.EnabledBPFBatchDelete = true
	}
	klog.Infof("EnabledBPFBatchDelete: %v", config.EnabledBPFBatchDelete)

	// set redfish credential file path
	if *redfishCredFilePath != "" {
		config.SetRedfishCredFilePath(*redfishCredFilePath)
	}

	config.LogConfigs()

	components.InitPowerImpl()
	platform.InitPowerImpl()

	collector_metric.InitAvailableParamAndMetrics()

	// For local estimator, there is endpoint provided, thus we should let
	// model component decide whether/how to init
	model.InitEstimateFunctions(collector_metric.ContainerMetricNames, collector_metric.NodeMetadataNames, collector_metric.NodeMetadataValues)

	if config.EnabledGPU {
		klog.Infof("Initializing the GPU collector")
		err := accelerator.Init()
		if err == nil {
			defer accelerator.Shutdown()
		} else {
			klog.Infof("Failed to initialize the GPU collector: %v", err)
		}
	}

	m := manager.New()
	prometheus.MustRegister(version.NewCollector("kepler_exporter"))
	prometheus.MustRegister(m.PrometheusCollector)
	defer m.MetricCollector.Destroy()
	defer components.StopPower()

	// starting a new gorotine to collect data and report metrics
	if err := m.Start(); err != nil {
		klog.Infof("%s", fmt.Sprintf("failed to start : %v", err))
	}
	metricPathConfig := config.GetMetricPath(*metricsPath)
	bindAddressConfig := config.GetBindAddress(*address)

	http.Handle(metricPathConfig, promhttp.Handler())
	http.HandleFunc("/healthz", healthProbe)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`<html>
                        <head><title>Energy Stats Exporter</title></head>
                        <body>
                        <h1>Energy Stats Exporter</h1>
                        <p><a href="` + metricPathConfig + `">Metrics</a></p>
                        </body>
                        </html>`))
		if err != nil {
			klog.Fatalf("%s", fmt.Sprintf("failed to write response: %v", err))
		}
	})

	startProfiling(*cpuProfile, *memProfile)

	ch := make(chan error)
	go func() {
		ch <- http.ListenAndServe(bindAddressConfig, nil)
	}()

	klog.Infof(startedMsg, time.Since(start))
	klog.Flush() // force flush to parse the start msg in the e2e test
	err := <-ch
	klog.Fatalf("%s", fmt.Sprintf("failed to bind on %s: %v", bindAddressConfig, err))
}
