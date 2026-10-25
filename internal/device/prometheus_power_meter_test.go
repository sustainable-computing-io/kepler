// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/sustainable-computing-io/kepler/config"
)

var _ CPUPowerMeter = (*PrometheusPowerMeter)(nil)

// newPromServer returns a test server that answers /api/v1/query with body.
func newPromServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" || r.URL.Query().Get("query") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func prometheusConfig(baseURL string, meters ...string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Cpu.PreferredMeters = meters
	cfg.Experimental = &config.Experimental{}
	cfg.Experimental.PrometheusPower = config.PrometheusPower{
		Enabled: ptr.To(true),
		BaseURL: baseURL,
		Query:   `node_power_watts{instance="n1"}`,
	}
	return cfg
}

const promSuccess = `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1700000000,"42.5"]}]}}`

func TestPrometheusClient_QueryPowerWatt(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    float64
		wantErr string
	}{
		{name: "string value", status: http.StatusOK, body: promSuccess, want: 42.5},
		{
			name:   "numeric value",
			status: http.StatusOK,
			body:   `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1700000000,7]}]}}`,
			want:   7,
		},
		{
			name:    "no samples",
			status:  http.StatusOK,
			body:    `{"status":"success","data":{"resultType":"vector","result":[]}}`,
			wantErr: "returned no samples",
		},
		{
			name:    "multiple series",
			status:  http.StatusOK,
			body:    `{"status":"success","data":{"resultType":"vector","result":[{"metric":{"psu":"1"},"value":[1700000000,"100"]},{"metric":{"psu":"2"},"value":[1700000000,"120"]}]}}`,
			wantErr: "returned 2 series, expected exactly 1",
		},
		{name: "http error", status: http.StatusInternalServerError, body: "", wantErr: "prometheus query failed"},
		{name: "invalid json", status: http.StatusOK, body: "{", wantErr: "failed to decode"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newPromServer(t, tc.status, tc.body)
			client := NewPrometheusClient(srv.URL+"/", "up")

			got, err := client.QueryPowerWatt(context.Background())
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tc.want, got, 1e-9)
		})
	}
}

func TestPrometheusPowerMeter_PowerAndZones(t *testing.T) {
	meter, err := NewPrometheusPowerMeter("", func(context.Context) (float64, error) { return 12.5, nil })
	require.NoError(t, err)
	require.NoError(t, meter.Init())

	zones, err := meter.Zones()
	require.NoError(t, err)
	require.Len(t, zones, 1)
	assert.Equal(t, "prometheus-node", zones[0].Name())
	assert.Equal(t, Energy(0), zones[0].MaxEnergy())

	power, err := zones[0].Power()
	require.NoError(t, err)
	assert.InDelta(t, 12.5, power.Watts(), 1e-9)
}

func TestPrometheusPowerMeter_NegativePowerFailsInit(t *testing.T) {
	meter, err := NewPrometheusPowerMeter("node", func(context.Context) (float64, error) { return -1, nil })
	require.NoError(t, err)
	assert.Error(t, meter.Init())
}

func TestCreateCPUMeter_Prometheus(t *testing.T) {
	srv := newPromServer(t, http.StatusOK, promSuccess)
	cfg := prometheusConfig(srv.URL, "prometheus")

	meter, err := CreateCPUMeter(discardLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "prometheus-power-meter", meter.Name())

	zone, err := meter.PrimaryEnergyZone()
	require.NoError(t, err)
	power, err := zone.Power()
	require.NoError(t, err)
	assert.InDelta(t, 42.5, power.Watts(), 1e-9)
}

func TestCreateCPUMeter_PrometheusUnreachable_FallsThroughToFake(t *testing.T) {
	srv := newPromServer(t, http.StatusServiceUnavailable, "")
	cfg := prometheusConfig(srv.URL, "prometheus", "fake")

	meter, err := CreateCPUMeter(discardLogger(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "fake-cpu-meter", meter.Name())
}

func TestBuildCPUMeter_Prometheus_RedactsCredentialsInLog(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	cfg := prometheusConfig("https://kepler:s3cret@vm.example.org:8428", "prometheus")

	_, err := buildCPUMeter("prometheus", logger, cfg)
	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "s3cret")
	assert.Contains(t, buf.String(), "kepler:xxxxx@vm.example.org")
}

func TestPrometheusClient_InvalidBaseURLDoesNotLeakCredentials(t *testing.T) {
	client := NewPrometheusClient("https://kepler:s3cret@vm.example.org:port", "up")

	_, err := client.QueryPowerWatt(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base URL")
	assert.NotContains(t, err.Error(), "s3cret")
}

func TestBuildCPUMeter_Prometheus_MissingConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	_, err := buildCPUMeter("prometheus", discardLogger(), cfg)
	assert.ErrorContains(t, err, "not configured")

	cfg = prometheusConfig("", "prometheus")
	_, err = buildCPUMeter("prometheus", discardLogger(), cfg)
	assert.ErrorContains(t, err, "baseURL or query is empty")
}
