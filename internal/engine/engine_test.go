// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/service"
)

type testService struct {
	name     string
	initErr  error
	shutdown func() error
}

func (s *testService) Name() string    { return s.name }
func (s *testService) Init() error     { return s.initErr }
func (s *testService) Shutdown() error { return s.shutdown() }

type testRunner struct {
	*testService
	run func(context.Context) error
}

func (s *testRunner) Run(ctx context.Context) error { return s.run(ctx) }

func TestInitializeUnwindsFailureInReverseOrder(t *testing.T) {
	var closed []string
	var services []service.Service
	for _, name := range []string{"first", "second", "failed"} {
		s := &testService{name: name, shutdown: func() error { closed = append(closed, name); return nil }}
		if name == "failed" {
			s.initErr = errors.New("initialization failed")
		}
		services = append(services, s)
	}
	e := &Engine{services: services}
	require.ErrorContains(t, e.initialize(), "initialization failed")
	require.Equal(t, []string{"failed", "second", "first"}, closed)
}

func TestCloseWaitsForRunnersAndCleansNonRunners(t *testing.T) {
	stopped := make(chan struct{})
	release := make(chan struct{})
	started := make(chan struct{})
	var cleanups atomic.Int32
	nonRunner := &testService{name: "dependency", shutdown: func() error {
		select {
		case <-stopped:
		default:
			return errors.New("closed while runner active")
		}
		cleanups.Add(1)
		return nil
	}}
	runner := &testRunner{testService: &testService{name: "runner", shutdown: func() error { cleanups.Add(1); return nil }}, run: func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		<-release
		close(stopped)
		return nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), services: []service.Service{nonRunner, runner}, cancel: cancel, done: make(chan struct{})}
	require.NoError(t, e.initialize())
	go e.run(ctx)
	<-started
	deadline, stop := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer stop()
	require.ErrorIs(t, e.Close(deadline), context.DeadlineExceeded)
	require.Zero(t, cleanups.Load())
	close(release)
	require.NoError(t, e.Close(context.Background()))
	require.NoError(t, e.Close(context.Background()))
	require.EqualValues(t, 2, cleanups.Load())
}

func TestRunnerFailureStopsRuntime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), cancel: cancel, done: make(chan struct{})}
	failure := errors.New("sampling failed")
	e.services = []service.Service{&testRunner{testService: &testService{name: "runner", shutdown: func() error { return nil }}, run: func(context.Context) error { return failure }}}
	require.NoError(t, e.initialize())
	go e.run(ctx)
	require.ErrorIs(t, e.Run(context.Background()), failure)
	require.ErrorIs(t, e.Close(context.Background()), failure)
	require.Error(t, e.IsAlive())
}
