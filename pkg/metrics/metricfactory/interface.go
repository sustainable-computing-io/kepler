/*
Copyright 2023.

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

package metricfactory

import (
	"github.com/prometheus/client_golang/prometheus"
)

type PromMetric interface {
	Desc() *prometheus.Desc
	MustMetric(value float64, labelValues ...string) prometheus.Metric
}

type promCounter struct {
	desc *prometheus.Desc
}

func NewPromCounter(desc *prometheus.Desc) PromMetric {
	return &promCounter{desc: desc}
}

func (c *promCounter) Desc() *prometheus.Desc {
	return c.desc
}

func (c *promCounter) MustMetric(value float64, labelValues ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(c.desc, prometheus.CounterValue, value, labelValues...)
}

type bpfGauge struct {
	desc *prometheus.Desc
}

func NewPromGauge(desc *prometheus.Desc) PromMetric {
	return &bpfGauge{desc: desc}
}

func (g *bpfGauge) Desc() *prometheus.Desc {
	return g.desc
}

func (g *bpfGauge) MustMetric(value float64, labelValues ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(g.desc, prometheus.GaugeValue, value, labelValues...)
}
