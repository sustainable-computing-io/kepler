// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

// Package common provides shared utilities for e2e tests.
package common

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// Metric represents a parsed Prometheus metric.
type Metric struct {
	Name   string
	Labels map[string]string
	Value  float64
}

// MetricFamily represents a group of metrics with the same name.
type MetricFamily struct {
	Name    string
	Metrics []Metric
}

// MetricsScraper scrapes Prometheus metrics from a URL.
type MetricsScraper struct {
	url    string
	client *http.Client
}

// NewMetricsScraper creates a new MetricsScraper.
func NewMetricsScraper(url string) *MetricsScraper {
	return &MetricsScraper{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// URL returns the metrics endpoint URL.
func (s *MetricsScraper) URL() string {
	return s.url
}

// Scrape fetches all metrics from the endpoint.
func (s *MetricsScraper) Scrape() (map[string]*MetricFamily, error) {
	resp, err := s.client.Get(s.url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metrics: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return parseMetrics(resp.Body, resp.Header)
}

// ScrapeMetric fetches a specific metric by name.
func (s *MetricsScraper) ScrapeMetric(name string) ([]Metric, error) {
	families, err := s.Scrape()
	if err != nil {
		return nil, err
	}

	family, ok := families[name]
	if !ok {
		return nil, fmt.Errorf("metric %s not found", name)
	}
	return family.Metrics, nil
}

// ScrapeMetricWithLabels fetches metrics matching the given labels.
func (s *MetricsScraper) ScrapeMetricWithLabels(name string, labels map[string]string) ([]Metric, error) {
	metrics, err := s.ScrapeMetric(name)
	if err != nil {
		return nil, err
	}

	var matched []Metric
	for _, m := range metrics {
		if MatchLabels(m.Labels, labels) {
			matched = append(matched, m)
		}
	}
	return matched, nil
}

// GetMetricValue returns a single metric value matching the labels.
func (s *MetricsScraper) GetMetricValue(name string, labels map[string]string) (float64, error) {
	metrics, err := s.ScrapeMetricWithLabels(name, labels)
	if err != nil {
		return 0, err
	}
	if len(metrics) == 0 {
		return 0, fmt.Errorf("no metrics found for %s", name)
	}
	if len(metrics) > 1 {
		return 0, fmt.Errorf("multiple metrics found for %s", name)
	}
	return metrics[0].Value, nil
}

// SumMetricValues returns sum of matching metrics.
func (s *MetricsScraper) SumMetricValues(name string, labels map[string]string) (float64, error) {
	metrics, err := s.ScrapeMetricWithLabels(name, labels)
	if err != nil {
		return 0, err
	}

	var sum float64
	for _, m := range metrics {
		sum += m.Value
	}
	return sum, nil
}

// TakeSnapshot captures current metrics as a point-in-time snapshot.
func (s *MetricsScraper) TakeSnapshot() (*MetricsSnapshot, error) {
	families, err := s.Scrape()
	if err != nil {
		return nil, err
	}
	return &MetricsSnapshot{Timestamp: time.Now(), Families: families}, nil
}

// MetricsSnapshot is a point-in-time capture of metrics.
type MetricsSnapshot struct {
	Timestamp time.Time
	Families  map[string]*MetricFamily
}

// GetValue returns a metric value from snapshot matching the labels.
func (ms *MetricsSnapshot) GetValue(name string, labels map[string]string) (float64, bool) {
	family, ok := ms.Families[name]
	if !ok {
		return 0, false
	}
	for _, m := range family.Metrics {
		if MatchLabels(m.Labels, labels) {
			return m.Value, true
		}
	}
	return 0, false
}

// SumValues returns sum of matching metrics from snapshot.
func (ms *MetricsSnapshot) SumValues(name string, labels map[string]string) float64 {
	family, ok := ms.Families[name]
	if !ok {
		return 0
	}
	var sum float64
	for _, m := range family.Metrics {
		if MatchLabels(m.Labels, labels) {
			sum += m.Value
		}
	}
	return sum
}

// HasMetric returns true if metric exists in snapshot.
func (ms *MetricsSnapshot) HasMetric(name string) bool {
	_, ok := ms.Families[name]
	return ok
}

// HasMetricWithLabels returns true if metric with matching labels exists.
func (ms *MetricsSnapshot) HasMetricWithLabels(name string, labels map[string]string) bool {
	family, ok := ms.Families[name]
	if !ok {
		return false
	}
	for _, m := range family.Metrics {
		if MatchLabels(m.Labels, labels) {
			return true
		}
	}
	return false
}

// GetAllWithName returns all metrics with the given name.
func (ms *MetricsSnapshot) GetAllWithName(name string) []Metric {
	family, ok := ms.Families[name]
	if !ok {
		return nil
	}
	return family.Metrics
}

// MatchLabels returns true if metricLabels contains all expected labels.
func MatchLabels(metricLabels, expected map[string]string) bool {
	for k, v := range expected {
		if metricLabels[k] != v {
			return false
		}
	}
	return true
}

// parseMetrics parses Prometheus metrics from response body.
func parseMetrics(r io.Reader, header http.Header) (map[string]*MetricFamily, error) {
	families := make(map[string]*MetricFamily)

	format := expfmt.ResponseFormat(header)
	decoder := expfmt.NewDecoder(r, format)

	for {
		var mf dto.MetricFamily
		if err := decoder.Decode(&mf); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode metrics: %w", err)
		}

		name := mf.GetName()
		family := &MetricFamily{Name: name}

		for _, m := range mf.GetMetric() {
			labels := make(map[string]string)
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}

			value := extractMetricValue(m)
			family.Metrics = append(family.Metrics, Metric{
				Name:   name,
				Labels: labels,
				Value:  value,
			})
		}

		families[name] = family
	}

	return families, nil
}

// extractMetricValue extracts the numeric value from a metric based on its type.
func extractMetricValue(m *dto.Metric) float64 {
	if g := m.GetGauge(); g != nil {
		return g.GetValue()
	}
	if c := m.GetCounter(); c != nil {
		return c.GetValue()
	}
	if u := m.GetUntyped(); u != nil {
		return u.GetValue()
	}
	if h := m.GetHistogram(); h != nil {
		return float64(h.GetSampleCount())
	}
	if s := m.GetSummary(); s != nil {
		return float64(s.GetSampleCount())
	}
	return 0
}

// WaitForCondition is a generic helper that waits for a condition to become true.
func WaitForCondition(ctx context.Context, interval time.Duration, check func() bool) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if check() {
				return nil
			}
		}
	}
}
