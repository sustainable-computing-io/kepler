// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"log/slog"
	"time"

	"k8s.io/utils/clock"
)

type Opts struct {
	logger       *slog.Logger
	sysfsPath    string
	interval     time.Duration
	clock        clock.WithTicker
	maxStaleness time.Duration
}

// NewConfig returns a new Config with defaults set
func DefaultOpts() Opts {
	return Opts{
		logger:       slog.Default(),
		sysfsPath:    "/sys",
		interval:     0 * time.Second, // no collection
		clock:        clock.RealClock{},
		maxStaleness: 500 * time.Millisecond,
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
