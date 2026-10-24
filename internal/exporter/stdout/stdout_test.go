// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package stdout

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/cpu"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
)

// MockMonitor mocks the Monitor interface
type MockMonitor struct {
	mock.Mock
}

func (m *MockMonitor) Init() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockMonitor) Run(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockMonitor) Shutdown() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockMonitor) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockMonitor) Snapshot() (*monitor.Snapshot, error) {
	args := m.Called()
	if s := args.Get(0); s != nil {
		return s.(*monitor.Snapshot), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockMonitor) DataChannel() <-chan struct{} {
	args := m.Called()
	return args.Get(0).(<-chan struct{})
}

func (m *MockMonitor) ZoneNames() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func TestNewExporter(t *testing.T) {
	tests := []struct {
		name          string
		expectService string
		opts          []OptionFn
		out           io.WriteCloser
		interval      time.Duration
	}{{
		name:          "default options",
		expectService: "stdout",
		opts:          []OptionFn{},
		out:           os.Stdout,
		interval:      2 * time.Second,
	}, {
		name:          "custom options",
		expectService: "stdout",
		opts: []OptionFn{
			WithLogger(slog.Default()),
			WithOutput(os.Stderr),
			WithInterval(20 * time.Second),
		},
		out:      os.Stderr,
		interval: 20 * time.Second,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMonitor := &MockMonitor{}
			exporter := NewExporter(mockMonitor, tt.opts...)
			assert.NotNil(t, exporter)
			assert.Equal(t, tt.expectService, exporter.Name())
			assert.NotNil(t, exporter.logger)
			assert.Same(t, mockMonitor, exporter.monitor)
			assert.Same(t, tt.out, exporter.out)
			assert.Equal(t, tt.interval, exporter.interval)
		})
	}
}

type dummyWriteCloser struct {
	io.Writer
}

func (dwc *dummyWriteCloser) Close() error {
	return nil
}

func TestExporter_InitRunShotdown(t *testing.T) {
	t.Run("starts successfully", func(t *testing.T) {
		mockMonitor := &MockMonitor{}
		mockMonitor.On("Snapshot").Return(&monitor.Snapshot{Node: getTestNodeData()}, nil)
		out := &dummyWriteCloser{&bytes.Buffer{}}
		exporter := NewExporter(mockMonitor, WithOutput(out), WithInterval(100*time.Millisecond))
		err := exporter.Init()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		go func() {
			_ = exporter.Run(ctx)
		}()
		assert.NoError(t, err)
		time.Sleep(2 * time.Second)
		cancel()
		assert.NoError(t, exporter.Shutdown())
		mockMonitor.AssertExpectations(t)
	})
}

// syncBuffer is a thread-safe io.WriteCloser used to read the exporter output
// while Run is writing to it
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *syncBuffer) Close() error {
	return nil
}

func TestExporter_RunContinuesAfterSnapshotError(t *testing.T) {
	mockMonitor := &MockMonitor{}
	mockMonitor.On("Snapshot").Return(nil, errors.New("snapshot failed")).Once()
	mockMonitor.On("Snapshot").Return(getTestNodeSnapshot(), nil)

	out := &syncBuffer{}
	exporter := NewExporter(mockMonitor, WithOutput(out), WithInterval(10*time.Millisecond))
	assert.NoError(t, exporter.Init())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- exporter.Run(ctx)
	}()

	// the snapshot after the failed one must still be written
	assert.Eventually(t, func() bool {
		return strings.Contains(out.String(), "package")
	}, 2*time.Second, 10*time.Millisecond, "snapshot after a failed one was not written")

	// Run must keep running until its context is cancelled
	select {
	case err := <-errCh:
		t.Fatalf("Run returned before the context was cancelled: %v", err)
	default:
	}

	cancel()
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
	mockMonitor.AssertExpectations(t)
}

func Test_print(t *testing.T) {
	buf := bytes.Buffer{}
	now, err := time.Parse(time.RFC3339, "2025-05-15T01:01:01Z")
	assert.NoError(t, err, "unexpected time parse error")
	write(&buf, now, getTestNodeSnapshot())
	expected := `
┌─────────┬─────────────┬────────────────┐
│  ZONE   │ POWER ( W ) │ ABSOLUTE ( J ) │
├─────────┼─────────────┼────────────────┤
│    dram │       2.00W │       2340.00J │
│ package │      12.00W │      12300.00J │
└─────────┴─────────────┴────────────────┘
`
	expected = strings.TrimLeft(expected, "\n")
	assert.Equal(t, expected, buf.String())
}

func getTestNodeSnapshot() *monitor.Snapshot {
	return &monitor.Snapshot{
		Node: getTestNodeData(),
	}
}

func getTestNodeData() *monitor.Node {
	// Setup test zones
	packageZone := cpu.NewMockRaplZone("package", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0", 1000)
	dramZone := cpu.NewMockRaplZone("dram", 0, "/sys/class/powercap/intel-rapl/intel-rapl:0:1", 1000)

	nodePkgAbs := 12300 * device.Joule
	nodePkgPower := 12 * device.Watt

	nodeDramAbs := 2340 * device.Joule
	nodeDramPower := 2 * device.Watt

	// Create test node Snapshot
	return &monitor.Node{
		Zones: monitor.NodeZoneUsageMap{
			packageZone: monitor.NodeUsage{
				EnergyTotal: nodePkgAbs,
				Power:       nodePkgPower,
			},
			dramZone: monitor.NodeUsage{
				EnergyTotal: nodeDramAbs,
				Power:       nodeDramPower,
			},
		},
	}
}
