// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/sustainable-computing-io/kepler/internal/version"
)

const (
	keplerNS       = "kepler"
	buildSubsystem = "build"
)

type BuildInfoCollector struct {
	buildInfo *prom.GaugeVec
}

// NewKeplerBuildInfoCollector creates a new collector for build information
func NewKeplerBuildInfoCollector() *BuildInfoCollector {
	buildInfo := prom.NewGaugeVec(
		prom.GaugeOpts{
			Namespace: keplerNS,
			Subsystem: buildSubsystem,
			Name:      "info",
			Help:      "A metric with a constant '1' value labeled with version information",
		},
		[]string{"arch", "branch", "revision", "version", "goversion"},
	)

	return &BuildInfoCollector{
		buildInfo: buildInfo,
	}
}

func (c *BuildInfoCollector) Describe(ch chan<- *prom.Desc) {
	c.buildInfo.Describe(ch)
}

func (c *BuildInfoCollector) Collect(ch chan<- prom.Metric) {
	info := version.Info()

	c.buildInfo.WithLabelValues(
		info.GoArch,
		info.GitBranch,
		info.GitCommit,
		info.Version,
		info.GoVersion,
	).Set(1)

	c.buildInfo.Collect(ch)
}
