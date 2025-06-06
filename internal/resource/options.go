// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"log/slog"
	"os"

	"github.com/sustainable-computing-io/kepler/internal/k8s/pod"
	"k8s.io/utils/clock"
)

// Options contains all the configuration for the ResourceTracker
type Options struct {
	logger      *slog.Logger
	clock       clock.Clock
	procFSPath  string
	procReader  allProcReader
	podInformer pod.Informer
}

// OptionFn is a function that configures the Options
type OptionFn func(*Options)

// WithProcFSPath sets the ProcReader
func WithProcFSPath(path string) OptionFn {
	return func(o *Options) {
		o.procFSPath = path
	}
}

// WithProcFSPath sets the ProcReader
func WithProcReader(r allProcReader) OptionFn {
	return func(o *Options) {
		o.procReader = r
	}
}

// WithPodInformer sets the pod informer
func WithPodInformer(pi pod.Informer) OptionFn {
	return func(o *Options) {
		o.podInformer = pi
	}
}

// WithLogger sets the logger
func WithLogger(logger *slog.Logger) OptionFn {
	return func(o *Options) {
		o.logger = logger
	}
}

// WithClock sets the clock implementation
func WithClock(c clock.Clock) OptionFn {
	return func(o *Options) {
		o.clock = c
	}
}

// defaultOptions returns the default options
func defaultOptions() *Options {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	return &Options{
		logger: logger,
		clock:  &clock.RealClock{},
	}
}
