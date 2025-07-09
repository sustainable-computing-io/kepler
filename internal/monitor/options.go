// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"log/slog"
	"time"

	"github.com/sustainable-computing-io/kepler/internal/resource"
	"k8s.io/utils/clock"
)

type Opts struct {
	logger                       *slog.Logger
	interval                     time.Duration
	clock                        clock.WithTicker
	resources                    resource.Informer
	maxStaleness                 time.Duration
	maxTerminated                int
	minTerminatedEnergyThreshold Energy
}

// NewConfig returns a new Config with defaults set
func DefaultOpts() Opts {
	return Opts{
		logger:                       slog.Default(),
		interval:                     5 * time.Second,
		clock:                        clock.RealClock{},
		maxStaleness:                 500 * time.Millisecond,
		resources:                    nil,
		maxTerminated:                500,
		minTerminatedEnergyThreshold: 10 * Joule,
	}
}

// OptionFn is a function sets one more more options in Opts struct
type OptionFn func(*Opts)

// WithInterval sets the interval for the PowerMonitor
func WithInterval(d time.Duration) OptionFn {
	return func(o *Opts) {
		o.interval = d
	}
}

// WithLogger sets the logger for the PowerMonitor
func WithLogger(logger *slog.Logger) OptionFn {
	return func(o *Opts) {
		o.logger = logger
	}
}

// WithClock sets the clock the PowerMonitor
func WithClock(c clock.WithTicker) OptionFn {
	return func(o *Opts) {
		o.clock = c
	}
}

// WithMaxStaleness sets the clock the PowerMonitor
func WithMaxStaleness(d time.Duration) OptionFn {
	return func(o *Opts) {
		o.maxStaleness = d
	}
}

// WithResourceInformer sets the resource informer for the PowerMonitor
func WithResourceInformer(r resource.Informer) OptionFn {
	return func(o *Opts) {
		o.resources = r
	}
}

// WithMaxTerminated sets the maximum number of terminated workloads to keep in memory
func WithMaxTerminated(max int) OptionFn {
	return func(o *Opts) {
		o.maxTerminated = max
	}
}

// WithMinTerminatedEnergyThreshold sets the minimum energy threshold for terminated workloads
func WithMinTerminatedEnergyThreshold(threshold Energy) OptionFn {
	return func(o *Opts) {
		o.minTerminatedEnergyThreshold = threshold
	}
}
