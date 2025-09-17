// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/sustainable-computing-io/kepler/internal/device"
)

func TestPowerMonitor_IsReady_PassiveMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	// Create monitor with interval = 0 (passive mode)
	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(0), // Passive mode
	)

	ctx := context.Background()

	// Should be ready immediately in passive mode
	ready, err := pm.IsReady(ctx)
	if err != nil {
		t.Errorf("Expected no error in passive mode, got: %v", err)
	}
	if !ready {
		t.Error("Expected ready=true in passive mode")
	}

}

func TestPowerMonitor_IsReady_ActiveMode_NoData(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	// Create monitor with active collection
	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(1*time.Second), // Active mode
	)

	ctx := context.Background()

	// Should not be ready without data
	ready, err := pm.IsReady(ctx)
	if err == nil {
		t.Error("Expected error when no data available")
	}
	if ready {
		t.Error("Expected ready=false when no data available")
	}
	if err.Error() != "no data yet" {
		t.Errorf("Expected 'no data yet' error, got: %v", err)
	}

}

func TestPowerMonitor_IsReady_ActiveMode_WithData(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	// Create monitor with active collection
	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(1*time.Second), // Active mode
	)

	// Manually create a snapshot to simulate data collection
	snapshot := NewSnapshot()
	snapshot.Timestamp = time.Now()
	pm.snapshot.Store(snapshot)

	ctx := context.Background()

	// Should be ready with data
	ready, err := pm.IsReady(ctx)
	if err != nil {
		t.Errorf("Expected no error with data available, got: %v", err)
	}
	if !ready {
		t.Error("Expected ready=true with data available")
	}

}

func TestPowerMonitor_IsReady_StateTransitions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(1*time.Second),
	)

	ctx := context.Background()

	// Initial state: not ready, never been ready
	ready, err := pm.IsReady(ctx)
	if ready || err == nil {
		t.Error("Expected not ready initially")
	}

	// Add data: should become ready
	snapshot := NewSnapshot()
	snapshot.Timestamp = time.Now()
	pm.snapshot.Store(snapshot)

	ready, err = pm.IsReady(ctx)
	if !ready || err != nil {
		t.Errorf("Expected ready with data, got ready=%v, err=%v", ready, err)
	}

	// Remove data: should not be ready, but HasBeenReady should remain true
	pm.snapshot.Store(nil)

	ready, err = pm.IsReady(ctx)
	if ready || err == nil {
		t.Error("Expected not ready after removing data")
	}

	// Add data again: should be ready again
	snapshot2 := NewSnapshot()
	snapshot2.Timestamp = time.Now()
	pm.snapshot.Store(snapshot2)

	ready, err = pm.IsReady(ctx)
	if !ready || err != nil {
		t.Errorf("Expected ready again with new data, got ready=%v, err=%v", ready, err)
	}
}

func TestPowerMonitor_IsReady_NilMonitor(t *testing.T) {
	var pm *PowerMonitor = nil
	ctx := context.Background()

	ready, err := pm.IsReady(ctx)
	if ready {
		t.Error("Expected ready=false for nil monitor")
	}
	if err == nil {
		t.Error("Expected error for nil monitor")
	}
	if err.Error() != "monitor is nil" {
		t.Errorf("Expected 'monitor is nil' error, got: %v", err)
	}
}


func TestPowerMonitor_IsReady_ConcurrentAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(1*time.Second),
	)

	// Add initial data
	snapshot := NewSnapshot()
	snapshot.Timestamp = time.Now()
	pm.snapshot.Store(snapshot)

	ctx := context.Background()

	// Run concurrent readiness checks
	const numGoroutines = 100
	results := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			ready, err := pm.IsReady(ctx)
			results <- ready
			errors <- err
		}()
	}

	// Collect results
	readyCount := 0
	errorCount := 0

	for i := 0; i < numGoroutines; i++ {
		ready := <-results
		err := <-errors

		if err != nil {
			errorCount++
		} else if ready {
			readyCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("Expected no errors in concurrent access, got %d errors", errorCount)
	}

	if readyCount != numGoroutines {
		t.Errorf("Expected %d ready results, got %d", numGoroutines, readyCount)
	}

}

