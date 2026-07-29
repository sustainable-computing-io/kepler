// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sustainable-computing-io/kepler/internal/service"
)

// capturingAPI records the handlers registered against it so that tests can
// exercise the probe endpoints directly.
type capturingAPI struct {
	handlers map[string]http.Handler
	err      error
}

func newCapturingAPI() *capturingAPI {
	return &capturingAPI{handlers: map[string]http.Handler{}}
}

func (c *capturingAPI) Name() string { return "capturing-api" }

func (c *capturingAPI) Register(endpoint, summary, description string, handler http.Handler) error {
	if c.err != nil {
		return c.err
	}
	c.handlers[endpoint] = handler
	return nil
}

// plainService implements no health check interfaces
type plainService struct{ name string }

func (s *plainService) Name() string { return s.name }

// liveService reports liveness only
type liveService struct {
	name string
	err  error
}

func (s *liveService) Name() string   { return s.name }
func (s *liveService) IsAlive() error { return s.err }

// readyService reports readiness only
type readyService struct {
	name string
	err  error
}

func (s *readyService) Name() string   { return s.name }
func (s *readyService) IsReady() error { return s.err }

// bothService reports both liveness and readiness
type bothService struct {
	name     string
	aliveErr error
	readyErr error
}

func (s *bothService) Name() string   { return s.name }
func (s *bothService) IsAlive() error { return s.aliveErr }
func (s *bothService) IsReady() error { return s.readyErr }

var (
	_ service.LivenessChecker  = (*liveService)(nil)
	_ service.ReadinessChecker = (*readyService)(nil)
	_ service.LivenessChecker  = (*bothService)(nil)
	_ service.ReadinessChecker = (*bothService)(nil)
	_ APIService               = (*capturingAPI)(nil)
)

// newInitializedProbe returns a probe with its endpoints registered on a
// capturing API, ready to be queried.
func newInitializedProbe(t *testing.T, services ...service.Service) (*probe, *capturingAPI) {
	t.Helper()

	api := newCapturingAPI()
	p := NewProbe(slog.New(slog.DiscardHandler), api, services)
	require.NoError(t, p.Init())
	require.Contains(t, api.handlers, LivezEndpoint)
	require.Contains(t, api.handlers, ReadyzEndpoint)

	return p, api
}

// get issues a GET against the registered handler for endpoint
func get(t *testing.T, api *capturingAPI, endpoint string) (int, probeResponse) {
	t.Helper()
	return do(t, api, http.MethodGet, endpoint)
}

func do(t *testing.T, api *capturingAPI, method, endpoint string) (int, probeResponse) {
	t.Helper()

	handler, ok := api.handlers[endpoint]
	require.True(t, ok, "no handler registered for %s", endpoint)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(method, endpoint, nil))

	var resp probeResponse
	if rr.Body.Len() > 0 {
		// a rejected method returns plain text, not JSON
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	}
	return rr.Code, resp
}

// markRunning starts the probe's Run loop and returns a func that stops it and
// waits for Run to return.
func markRunning(t *testing.T, p *probe) (stop func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		assert.NoError(t, p.Run(ctx))
	}()

	require.Eventually(t, p.running.Load, time.Second, time.Millisecond,
		"probe should be marked running once Run is called")

	return func() {
		cancel()
		<-done
	}
}

func TestProbeName(t *testing.T) {
	p, _ := newInitializedProbe(t)
	assert.Equal(t, "probe", p.Name())
}

func TestProbeInitRegistersEndpoints(t *testing.T) {
	api := newCapturingAPI()
	p := NewProbe(slog.New(slog.DiscardHandler), api, nil)

	require.NoError(t, p.Init())

	assert.Equal(t, []string{LivezEndpoint, ReadyzEndpoint}, sortedKeys(api.handlers))
	assert.Equal(t, "/probe/livez", LivezEndpoint)
	assert.Equal(t, "/probe/readyz", ReadyzEndpoint)
}

func TestProbeInitPropagatesRegisterError(t *testing.T) {
	api := newCapturingAPI()
	api.err = errors.New("register failed")

	p := NewProbe(slog.New(slog.DiscardHandler), api, nil)

	err := p.Init()
	assert.ErrorContains(t, err, "register failed")
}

// Liveness must not depend on kepler having started: answering the request at
// all proves the process is not dead-locked.
func TestLivezIsHealthyBeforeAndAfterRun(t *testing.T) {
	p, api := newInitializedProbe(t)

	code, resp := get(t, api, LivezEndpoint)
	assert.Equal(t, http.StatusOK, code, "livez should be healthy before Run")
	assert.Equal(t, statusAlive, resp.Status)
	assert.Empty(t, resp.Failures)

	stop := markRunning(t, p)
	code, resp = get(t, api, LivezEndpoint)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "alive", resp.Status)

	stop()
	code, resp = get(t, api, LivezEndpoint)
	assert.Equal(t, http.StatusOK, code, "livez should stay healthy while shutting down")
	assert.Equal(t, "alive", resp.Status)
}

// Readiness is gated on every service having been initialized and started,
// which is exactly when the probe's Run is called.
func TestReadyzBecomesReadyOnRun(t *testing.T) {
	p, api := newInitializedProbe(t)

	code, resp := get(t, api, ReadyzEndpoint)
	assert.Equal(t, http.StatusServiceUnavailable, code, "readyz must fail before Run")
	assert.Equal(t, statusUnavailable, resp.Status)
	assert.Equal(t, map[string]string{"kepler": "services have not started yet"}, resp.Failures)

	stop := markRunning(t, p)
	code, resp = get(t, api, ReadyzEndpoint)
	assert.Equal(t, http.StatusOK, code, "readyz must succeed once running")
	assert.Equal(t, "ok", resp.Status)
	assert.Empty(t, resp.Failures)

	stop()
	code, resp = get(t, api, ReadyzEndpoint)
	assert.Equal(t, http.StatusServiceUnavailable, code, "readyz must fail again after shutdown")
	assert.Equal(t, statusUnavailable, resp.Status)
}

func TestShutdownMarksNotReady(t *testing.T) {
	p, api := newInitializedProbe(t)

	stop := markRunning(t, p)
	t.Cleanup(stop)

	code, _ := get(t, api, ReadyzEndpoint)
	require.Equal(t, http.StatusOK, code)

	require.NoError(t, p.Shutdown())

	code, resp := get(t, api, ReadyzEndpoint)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, statusUnavailable, resp.Status)
}

// Services implementing neither interface must not influence the verdict.
func TestServicesWithoutChecksAreIgnored(t *testing.T) {
	p, api := newInitializedProbe(t,
		&plainService{name: "monitor"},
		&plainService{name: "resource-informer"},
	)
	assert.Empty(t, p.liveChecks)
	assert.Empty(t, p.readyChecks)

	stop := markRunning(t, p)
	t.Cleanup(stop)

	code, _ := get(t, api, LivezEndpoint)
	assert.Equal(t, http.StatusOK, code)
	code, _ = get(t, api, ReadyzEndpoint)
	assert.Equal(t, http.StatusOK, code)
}

func TestCheckersAreDiscoveredByInterface(t *testing.T) {
	p, _ := newInitializedProbe(t,
		&plainService{name: "plain"},
		&liveService{name: "live-only"},
		&readyService{name: "ready-only"},
		&bothService{name: "both"},
	)

	assert.Equal(t, []string{"live-only", "both"}, checkNames(p.liveChecks))
	assert.Equal(t, []string{"ready-only", "both"}, checkNames(p.readyChecks))
}

func TestFailingCheckersReportUnavailable(t *testing.T) {
	tests := []struct {
		name     string
		services []service.Service
		endpoint string
		failures map[string]string
	}{{
		name:     "failing liveness checker",
		services: []service.Service{&liveService{name: "monitor", err: errors.New("collection stalled")}},
		endpoint: LivezEndpoint,
		failures: map[string]string{"monitor": "collection stalled"},
	}, {
		name:     "failing readiness checker",
		services: []service.Service{&readyService{name: "pod-informer", err: errors.New("cache not synced")}},
		endpoint: ReadyzEndpoint,
		failures: map[string]string{"pod-informer": "cache not synced"},
	}, {
		name: "failures from several services are aggregated",
		services: []service.Service{
			&liveService{name: "monitor", err: errors.New("collection stalled")},
			&liveService{name: "healthy", err: nil},
			&bothService{name: "redfish", aliveErr: errors.New("bmc unreachable")},
		},
		endpoint: LivezEndpoint,
		failures: map[string]string{"monitor": "collection stalled", "redfish": "bmc unreachable"},
	}, {
		name: "readiness failure is independent of liveness failure",
		services: []service.Service{
			&bothService{name: "redfish", aliveErr: errors.New("bmc unreachable")},
		},
		endpoint: ReadyzEndpoint,
		failures: nil,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, api := newInitializedProbe(t, tt.services...)
			stop := markRunning(t, p)
			t.Cleanup(stop)

			code, resp := get(t, api, tt.endpoint)

			if tt.failures == nil {
				assert.Equal(t, http.StatusOK, code)
				assert.Empty(t, resp.Failures)
				return
			}

			assert.Equal(t, http.StatusServiceUnavailable, code)
			assert.Equal(t, statusUnavailable, resp.Status)
			assert.Equal(t, tt.failures, resp.Failures)
		})
	}
}

func TestProbeResponseContentType(t *testing.T) {
	p, api := newInitializedProbe(t)
	stop := markRunning(t, p)
	t.Cleanup(stop)

	for _, endpoint := range []string{LivezEndpoint, ReadyzEndpoint} {
		t.Run(endpoint, func(t *testing.T) {
			rr := httptest.NewRecorder()
			api.handlers[endpoint].ServeHTTP(rr, httptest.NewRequest(http.MethodGet, endpoint, nil))

			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestProbeRejectsNonReadMethods(t *testing.T) {
	p, api := newInitializedProbe(t)
	stop := markRunning(t, p)
	t.Cleanup(stop)

	for _, endpoint := range []string{LivezEndpoint, ReadyzEndpoint} {
		t.Run(endpoint, func(t *testing.T) {
			code, _ := do(t, api, http.MethodHead, endpoint)
			assert.Equal(t, http.StatusOK, code, "HEAD should be accepted")

			for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
				rr := httptest.NewRecorder()
				api.handlers[endpoint].ServeHTTP(rr, httptest.NewRequest(method, endpoint, nil))

				assert.Equal(t, http.StatusMethodNotAllowed, rr.Code, "%s should be rejected", method)
				assert.Equal(t, "GET, HEAD", rr.Header().Get("Allow"))
			}
		})
	}
}

// The probes are served concurrently with the run/shutdown transitions, so the
// handlers and the running flag must be race free.
func TestProbeConcurrentAccess(t *testing.T) {
	p, api := newInitializedProbe(t,
		&bothService{name: "monitor"},
		&liveService{name: "redfish", err: errors.New("bmc unreachable")},
	)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 25 {
				code, _ := get(t, api, LivezEndpoint)
				assert.Equal(t, http.StatusServiceUnavailable, code)
				code, _ = get(t, api, ReadyzEndpoint)
				assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, code)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 25 {
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				defer close(done)
				assert.NoError(t, p.Run(ctx))
			}()
			cancel()
			<-done
		}
	}()

	wg.Wait()
}

// TestProbeEndToEnd serves the probes through the real APIServer over HTTP, as
// a kubelet probe would reach them.
func TestProbeEndToEnd(t *testing.T) {
	addr := fmt.Sprintf("127.0.0.1:%d", findFreePort())

	apiServer := NewAPIServer(WithListenAddress([]string{addr}))
	require.NoError(t, apiServer.Init())

	unhealthy := &bothService{name: "monitor"}
	p := NewProbe(slog.New(slog.DiscardHandler), apiServer, []service.Service{unhealthy})
	require.NoError(t, p.Init())

	ctx, cancel := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() { serverDone <- apiServer.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-serverDone
		_ = apiServer.Shutdown()
	})

	probeURL := func(endpoint string) string { return "http://" + addr + endpoint }

	// wait for the listener to accept connections
	require.Eventually(t, func() bool {
		resp, err := http.Get(probeURL(LivezEndpoint))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return true
	}, 3*time.Second, 10*time.Millisecond, "server should start serving")

	fetch := func(t *testing.T, endpoint string) (int, probeResponse) {
		t.Helper()
		resp, err := http.Get(probeURL(endpoint))
		require.NoError(t, err)
		defer resp.Body.Close()

		var body probeResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		return resp.StatusCode, body
	}

	// not started yet: alive but not ready
	code, body := fetch(t, LivezEndpoint)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "alive", body.Status)

	code, body = fetch(t, ReadyzEndpoint)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "unavailable", body.Status)

	stop := markRunning(t, p)
	t.Cleanup(stop)

	code, body = fetch(t, ReadyzEndpoint)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body.Status)

	// a service reporting unhealthy is surfaced over HTTP
	unhealthy.readyErr = errors.New("terminated workload cache full")
	code, body = fetch(t, ReadyzEndpoint)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, map[string]string{"monitor": "terminated workload cache full"}, body.Failures)

	// the probes are listed on the landing page
	resp, err := http.Get("http://" + addr + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	landing, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(landing), LivezEndpoint)
	assert.Contains(t, string(landing), ReadyzEndpoint)
}

func checkNames(checks []namedCheck) []string {
	names := make([]string, 0, len(checks))
	for _, c := range checks {
		names = append(names, c.name)
	}
	return names
}

func sortedKeys(m map[string]http.Handler) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// only ever two keys; keep the assertion order stable
	if len(keys) == 2 && keys[0] > keys[1] {
		keys[0], keys[1] = keys[1], keys[0]
	}
	return keys
}
