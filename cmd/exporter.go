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
	"log"
	"net/http"

	"github.com/sustainable-computing-io/kepler/pkg/collector"
	"github.com/sustainable-computing-io/kepler/pkg/rapl"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
)

var (
	address     = flag.String("address", "0.0.0.0:8888", "bind address")
	metricsPath = flag.String("metrics-path", "/metrics", "metrics path")
)

func main() {
	flag.Parse()

	err := prometheus.Register(version.NewCollector("energy_stats_exporter"))
	if err != nil {
		log.Fatalf("failed to register : %v", err)
	}

	collector, err := collector.New()
	if err != nil {
		log.Fatalf("failed to create collector: %v", err)
	}
	err = collector.Attach()
	if err != nil {
		log.Fatalf("failed to attach : %v", err)
	}
	defer collector.Destroy()
	defer rapl.StopRAPL()
	err = prometheus.Register(collector)
	if err != nil {
		log.Fatalf("failed to register collector: %v", err)
	}

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err = w.Write([]byte(`<html>
			<head><title>Energy Stats Exporter</title></head>
			<body>
			<h1>Energy Stats Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
		if err != nil {
			log.Fatalf("failed to write response: %v", err)
		}
	})

	err = http.ListenAndServe(*address, nil)
	if err != nil {
		log.Fatalf("failed to bind on %s: %v", *address, err)
	}
}
