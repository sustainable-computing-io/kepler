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
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/collector"
	"github.com/sustainable-computing-io/kepler/pkg/config"
	"github.com/sustainable-computing-io/kepler/pkg/power/gpu"
	"github.com/sustainable-computing-io/kepler/pkg/power/rapl"

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
	modelServerEndpoint          = flag.String("model-server-endpoint", "", "model server endpoint")
	enabledEBPFCgroupID          = flag.Bool("enable-cgroup-id", true, "whether enable eBPF to collect cgroup id (must have kernel version >= 4.18 and cGroup v2)")
	exposeHardwareCounterMetrics = flag.Bool("expose-hardware-counter-metrics", true, "whether expose hardware counter as prometheus metrics")
)

func healthProbe(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`ok`))
	if err != nil {
		klog.Fatalf("%s", fmt.Sprintf("failed to write response: %v", err))
	}
}

func finalizing() {
	exitCode := 10
	klog.Infoln(finishingMsg)
	klog.FlushAndExit(klog.ExitFlushTimeout, exitCode)
}

func main() {
	start := time.Now()
	defer finalizing()
	klog.InitFlags(nil)
	flag.Parse()

	config.EnableEBPFCgroupID(*enabledEBPFCgroupID)
	config.EnableHardwareCounterMetrics(*exposeHardwareCounterMetrics)

	collector.SetEnabledMetrics()

	err := prometheus.Register(version.NewCollector("energy_stats_exporter"))
	if err != nil {
		klog.Fatalf("failed to register : %v", err)
	}

	if *enableGPU {
		err = gpu.Init()
		if err == nil {
			defer gpu.Shutdown()
		}
	}

	if modelServerEndpoint != nil {
		config.SetModelServerEndpoint(*modelServerEndpoint)
	}

	newCollector, err := collector.New()
	defer newCollector.Destroy()
	defer rapl.StopPower()
	if err != nil {
		klog.Fatalf("%s", fmt.Sprintf("failed to create collector: %v", err))
	}
	err = newCollector.Attach()
	if err != nil {
		klog.Infof("%s", fmt.Sprintf("failed to attach : %v", err))
	}

	err = prometheus.Register(newCollector)
	if err != nil {
		klog.Fatalf("%s", fmt.Sprintf("failed to register collector: %v", err))
	}

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/healthz", healthProbe)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err = w.Write([]byte(`<html>
			<head><title>Energy Stats Exporter</title></head>
			<body>
			<h1>Energy Stats Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
		if err != nil {
			klog.Fatalf("%s", fmt.Sprintf("failed to write response: %v", err))
		}
	})

	ch := make(chan error)
	go func() {
		ch <- http.ListenAndServe(*address, nil)
	}()

	klog.Infof(startedMsg, time.Since(start))
	klog.Flush() // force flush to parse the start msg in the e2e test
	err = <-ch
	klog.Fatalf("%s", fmt.Sprintf("failed to bind on %s: %v", *address, err))
}
