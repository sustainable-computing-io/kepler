// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

// Package engine owns Kepler's sampling services independently of transports.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/platform/redfish"
	"github.com/sustainable-computing-io/kepler/internal/service"
	"k8s.io/utils/ptr"
)

// Engine owns the initialized dependencies and background goroutines of one
// Kepler instance. Config is an owned copy; it is immutable after construction.
type Engine struct {
	Config      *config.Config
	Monitor     *monitor.PowerMonitor
	Platform    *redfish.Service
	logger      *slog.Logger
	services    []service.Service
	initialized []service.Service
	cancel      context.CancelFunc
	done        chan struct{}
	err         error // written before closing done, read only after done
}

// New initializes and starts an instance without HTTP or signal handling.
func New(cfg *config.Config, logger *slog.Logger) (*Engine, error) {
	if cfg == nil {
		return nil, errors.New("nil Kepler configuration")
	}
	if logger == nil {
		logger = slog.Default()
	}
	cfg = cfg.Clone()
	cfg.ApplyCpuMeterDeprecations(logger)
	if err := cfg.ValidateRuntime(); err != nil {
		return nil, err
	}
	if cfg.Kube.Node == "" && !ptr.Deref(cfg.Kube.Enabled, false) {
		name, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("resolve node name: %w", err)
		}
		cfg.Kube.Node = name
	}
	if cfg.IsFeatureEnabled(config.ExperimentalRedfishFeature) && cfg.Experimental.Platform.Redfish.NodeName == "" {
		cfg.Experimental.Platform.Redfish.NodeName = cfg.Kube.Node
	}
	e, err := compose(logger, cfg)
	if err != nil {
		return nil, err
	}
	if err := e.initialize(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.done = make(chan struct{})
	go e.run(ctx)
	return e, nil
}

func (e *Engine) initialize() error {
	for _, svc := range e.services {
		if initializer, ok := svc.(service.Initializer); ok {
			if err := initializer.Init(); err != nil {
				// Include the failing service so partial initialization can be released.
				e.initialized = append(e.initialized, svc)
				return errors.Join(fmt.Errorf("initialize %s: %w", svc.Name(), err), shutdownServices(e.initialized))
			}
		}
		e.initialized = append(e.initialized, svc)
	}
	return nil
}

func (e *Engine) run(ctx context.Context) {
	var wg sync.WaitGroup
	// Every runner may report an error before the supervisor starts receiving.
	results := make(chan error, len(e.services))
	for _, svc := range e.services {
		if runner, ok := svc.(service.Runner); ok {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := runner.Run(ctx)
				if err != nil {
					err = fmt.Errorf("run %s: %w", svc.Name(), err)
				}
				results <- err
			}()
		}
	}
	select {
	case <-ctx.Done():
	case err := <-results:
		e.err = err
	}
	e.cancel()
	wg.Wait()
	close(results)
	for err := range results {
		e.err = errors.Join(e.err, err)
	}
	e.err = errors.Join(e.err, shutdownServices(e.initialized))
	if e.err != nil {
		e.logger.Error("Kepler runtime stopped", "error", e.err)
	}
	close(e.done)
}

func shutdownServices(services []service.Service) error {
	var err error
	for i := len(services) - 1; i >= 0; i-- {
		if closer, ok := services[i].(service.Shutdowner); ok {
			if closeErr := closer.Shutdown(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("shutdown %s: %w", services[i].Name(), closeErr))
			}
		}
	}
	return err
}

// Close cancels the runtime and waits for cleanup within ctx's deadline.
// Cleanup continues if ctx expires. Concurrent and repeated calls are safe.
func (e *Engine) Close(ctx context.Context) error {
	e.cancel()
	select {
	case <-e.done:
		return e.err
	default:
	}
	select {
	case <-e.done:
		return e.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Name, Run and Shutdown adapt the engine to the standalone service framework.
func (e *Engine) Init() error  { return e.IsAlive() }
func (e *Engine) Name() string { return "runtime" }
func (e *Engine) Run(ctx context.Context) error {
	select {
	case <-e.done:
		return e.err
	case <-ctx.Done():
		return nil
	}
}
func (e *Engine) Shutdown() error { return e.Close(context.Background()) }

// IsAlive and IsReady preserve health checks for the engine's services.
func (e *Engine) IsAlive() error {
	select {
	case <-e.done:
		return errors.Join(errors.New("runtime stopped"), e.err)
	default:
	}
	for _, svc := range e.services {
		if checker, ok := svc.(service.LivenessChecker); ok {
			if err := checker.IsAlive(); err != nil {
				return fmt.Errorf("%s: %w", svc.Name(), err)
			}
		}
	}
	return nil
}
func (e *Engine) IsReady() error {
	if err := e.IsAlive(); err != nil {
		return err
	}
	for _, svc := range e.services {
		if checker, ok := svc.(service.ReadinessChecker); ok {
			if err := checker.IsReady(); err != nil {
				return fmt.Errorf("%s: %w", svc.Name(), err)
			}
		}
	}
	return nil
}
