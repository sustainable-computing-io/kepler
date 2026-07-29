// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/sustainable-computing-io/kepler/internal/service"
)

const (
	// LivezEndpoint is the path of the liveness probe endpoint
	LivezEndpoint = "/probe/livez"
	// ReadyzEndpoint is the path of the readiness probe endpoint
	ReadyzEndpoint = "/probe/readyz"

	// statusAlive is reported by a successful liveness probe
	statusAlive = "alive"
	// statusOK is reported by a successful readiness probe
	statusOK = "ok"
	// statusUnavailable is reported by a failed probe
	statusUnavailable = "unavailable"
)

// probeResponse is the JSON body returned by both probe endpoints
type probeResponse struct {
	Status string `json:"status"`
	// Failures maps the name of each unhealthy service to the reason it reported
	Failures map[string]string `json:"failures,omitempty"`
}

// namedCheck is a health check of a single service, normalized so that liveness
// and readiness can share the same handler.
type namedCheck struct {
	name string
	run  func() error
}

// probe exposes Kubernetes liveness and readiness probe endpoints.
//
// The probes are deliberately cheap: unlike /metrics, hitting them performs no
// power computation, so they are safe to call on a short period. Kepler is
// considered alive as soon as it can answer HTTP, and ready once every service
// has been initialized and started, which is when this service's Run is called.
//
// Services that need a say in either verdict implement
// service.LivenessChecker or service.ReadinessChecker and are picked up here
// automatically.
type probe struct {
	logger *slog.Logger
	api    APIService

	liveChecks  []namedCheck
	readyChecks []namedCheck

	// running is set once Run is called, i.e. once all services are initialized
	// and started
	running atomic.Bool
}

var (
	_ service.Service     = (*probe)(nil)
	_ service.Initializer = (*probe)(nil)
	_ service.Runner      = (*probe)(nil)
)

// NewProbe creates the health probe service. services must contain every
// service kepler runs so that those implementing service.LivenessChecker or
// service.ReadinessChecker are reflected in the probe results.
func NewProbe(logger *slog.Logger, api APIService, services []service.Service) *probe {
	p := &probe{
		logger: logger.With("service", "probe"),
		api:    api,
	}

	for _, s := range services {
		if c, ok := s.(service.LivenessChecker); ok {
			p.liveChecks = append(p.liveChecks, namedCheck{name: c.Name(), run: c.IsAlive})
		}
		if c, ok := s.(service.ReadinessChecker); ok {
			p.readyChecks = append(p.readyChecks, namedCheck{name: c.Name(), run: c.IsReady})
		}
	}

	return p
}

func (p *probe) Name() string {
	return "probe"
}

func (p *probe) Init() error {
	p.logger.Debug("registering probe endpoints",
		"liveness-checkers", len(p.liveChecks),
		"readiness-checkers", len(p.readyChecks),
	)

	if err := p.api.Register(
		LivezEndpoint, "livez",
		"Liveness probe reporting whether kepler is running",
		p.handler(statusAlive, func() []namedCheck { return p.liveChecks }),
	); err != nil {
		return err
	}

	return p.api.Register(
		ReadyzEndpoint, "readyz",
		"Readiness probe reporting whether kepler is ready to serve",
		p.handler(statusOK, func() []namedCheck { return p.readyChecks }),
	)
}

// Run marks kepler as ready and blocks until the context is cancelled.
//
// service.Init has returned for every service by the time any Run is called, so
// reaching this point means initialization completed successfully.
func (p *probe) Run(ctx context.Context) error {
	p.running.Store(true)
	p.logger.Debug("kepler is ready")

	<-ctx.Done()
	p.running.Store(false)
	return nil
}

func (p *probe) Shutdown() error {
	p.running.Store(false)
	return nil
}

// handler answers a probe request with okStatus when every check passes, or 503
// and the failure reasons otherwise.
func (p *probe) handler(okStatus string, checks func() []namedCheck) http.Handler {
	// readiness additionally requires that all services have been started
	requireRunning := okStatus == statusOK

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		resp := probeResponse{Status: okStatus}
		code := http.StatusOK

		switch {
		case requireRunning && !p.running.Load():
			resp = probeResponse{
				Status:   statusUnavailable,
				Failures: map[string]string{"kepler": "services have not started yet"},
			}
			code = http.StatusServiceUnavailable

		default:
			var failures map[string]string
			for _, c := range checks() {
				if err := c.run(); err != nil {
					if failures == nil {
						failures = map[string]string{}
					}
					failures[c.name] = err.Error()
				}
			}
			if len(failures) > 0 {
				resp = probeResponse{Status: statusUnavailable, Failures: failures}
				code = http.StatusServiceUnavailable
			}
		}

		if code != http.StatusOK {
			p.logger.Warn("probe reported unhealthy", "endpoint", r.URL.Path, "failures", resp.Failures)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			p.logger.Error("failed to write probe response", "endpoint", r.URL.Path, "error", err)
		}
	})
}
