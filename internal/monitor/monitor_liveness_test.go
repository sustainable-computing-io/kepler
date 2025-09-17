// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sustainable-computing-io/kepler/internal/device"
	clocktesting "k8s.io/utils/clock/testing"
)

func TestPowerMonitor_IsLive_NilMonitor(t *testing.T) {
	var pm *PowerMonitor = nil
	ctx := context.Background()

	alive, err := pm.IsLive(ctx)
	if alive {
		t.Error("Expected alive=false for nil monitor")
	}
	if err == nil {
		t.Error("Expected error for nil monitor")
	}
	if err.Error() != "monitor is nil" {
		t.Errorf("Expected 'monitor is nil' error, got: %v", err)
	}
}

func TestPowerMonitor_IsLive_NilCPUMeter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	pm := NewPowerMonitor(
		nil, // nil CPU meter
		WithLogger(logger),
		WithInterval(0), // Passive mode
	)

	ctx := context.Background()

	alive, err := pm.IsLive(ctx)
	if alive {
		t.Error("Expected alive=false with nil CPU meter")
	}
	if err == nil {
		t.Error("Expected error with nil CPU meter")
	}
	if err.Error() != "CPU meter not initialized" {
		t.Errorf("Expected 'CPU meter not initialized' error, got: %v", err)
	}
}

func TestPowerMonitor_IsLive_PassiveMode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(0), // Passive mode - no heartbeat required
	)

	ctx := context.Background()

	// Should be alive immediately in passive mode (no heartbeat check)
	alive, err := pm.IsLive(ctx)
	if err != nil {
		t.Errorf("Expected no error in passive mode, got: %v", err)
	}
	if !alive {
		t.Error("Expected alive=true in passive mode")
	}
}

func TestPowerMonitor_IsLive_ActiveMode_NoHeartbeat(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(1*time.Second), // Active mode
	)

	ctx := context.Background()

	// Should not be alive without heartbeat
	alive, err := pm.IsLive(ctx)
	if alive {
		t.Error("Expected alive=false without heartbeat")
	}
	if err == nil {
		t.Error("Expected error without heartbeat")
	}
	if err.Error() != "no collection heartbeat yet" {
		t.Errorf("Expected 'no collection heartbeat yet' error, got: %v", err)
	}
}

func TestPowerMonitor_IsLive_ActiveMode_FreshHeartbeat(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(1*time.Second), // Active mode
	)

	// Simulate fresh heartbeat
	now := time.Now()
	atomic.StoreInt64(&pm.lastCollectUnixNano, now.UnixNano())

	ctx := context.Background()

	// Should be alive with fresh heartbeat
	alive, err := pm.IsLive(ctx)
	if err != nil {
		t.Errorf("Expected no error with fresh heartbeat, got: %v", err)
	}
	if !alive {
		t.Error("Expected alive=true with fresh heartbeat")
	}
}

func TestPowerMonitor_IsLive_ActiveMode_StaleHeartbeat(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	interval := 1 * time.Second
	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(interval), // Active mode
	)

	// Simulate stale heartbeat (older than 2*interval)
	staleTime := time.Now().Add(-3 * interval) // 3 seconds ago, should be stale
	atomic.StoreInt64(&pm.lastCollectUnixNano, staleTime.UnixNano())

	ctx := context.Background()

	// Should not be alive with stale heartbeat
	alive, err := pm.IsLive(ctx)
	if alive {
		t.Error("Expected alive=false with stale heartbeat")
	}
	if err == nil {
		t.Error("Expected error with stale heartbeat")
	}
	if !containsString(err.Error(), "collector stalled") {
		t.Errorf("Expected 'collector stalled' error, got: %v", err)
	}
}

func TestPowerMonitor_IsLive_ActiveMode_HeartbeatAtLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	interval := 1 * time.Second
	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(interval),
	)

	// Simulate heartbeat exactly at the tolerance limit (2*interval)
	limitTime := time.Now().Add(-2*interval + 100*time.Millisecond) // Just under the limit
	atomic.StoreInt64(&pm.lastCollectUnixNano, limitTime.UnixNano())

	ctx := context.Background()

	// Should still be alive just under the limit
	alive, err := pm.IsLive(ctx)
	if err != nil {
		t.Errorf("Expected no error just under limit, got: %v", err)
	}
	if !alive {
		t.Error("Expected alive=true just under limit")
	}
}

func TestPowerMonitor_IsLive_WithRealHeartbeatUpdate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	testClock := clocktesting.NewFakeClock(time.Now())
	
	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(1*time.Second),
		WithClock(testClock),
	)

	if err := pm.Init(); err != nil {
		t.Fatalf("Failed to init monitor: %v", err)
	}

	ctx := context.Background()

	// Initial state: no heartbeat
	alive, err := pm.IsLive(ctx)
	if alive || err == nil {
		t.Error("Expected not alive initially")
	}

	// Simulate a collection cycle by manually setting heartbeat
	// (calling refreshSnapshot would require full initialization which is complex for this test)
	now := testClock.Now()
	atomic.StoreInt64(&pm.lastCollectUnixNano, now.UnixNano())

	// Now should be alive after collection
	alive, err = pm.IsLive(ctx)
	if err != nil {
		t.Errorf("Expected alive after collection, got: %v", err)
	}
	if !alive {
		t.Error("Expected alive=true after collection")
	}

	// Advance real time by simulating a stale heartbeat
	// (We can't use testClock.Step because IsLive uses real time.Since, not pm.clock)
	staleTime := time.Now().Add(-3 * time.Second)
	atomic.StoreInt64(&pm.lastCollectUnixNano, staleTime.UnixNano())

	// Should now be stale
	alive, err = pm.IsLive(ctx)
	if alive {
		t.Error("Expected not alive after setting stale heartbeat")
	}
	if err == nil || !containsString(err.Error(), "collector stalled") {
		t.Errorf("Expected stalled error, got: %v", err)
	}
}

func TestPowerMonitor_IsLive_ConcurrentAccess(t *testing.T) {
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

	// Set fresh heartbeat
	now := time.Now()
	atomic.StoreInt64(&pm.lastCollectUnixNano, now.UnixNano())

	ctx := context.Background()

	// Run concurrent liveness checks
	const numGoroutines = 100
	results := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			alive, err := pm.IsLive(ctx)
			results <- alive
			errors <- err
		}()
	}

	// Collect results
	aliveCount := 0
	errorCount := 0

	for i := 0; i < numGoroutines; i++ {
		alive := <-results
		err := <-errors

		if err != nil {
			errorCount++
		} else if alive {
			aliveCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("Expected no errors in concurrent access, got %d errors", errorCount)
	}

	if aliveCount != numGoroutines {
		t.Errorf("Expected %d alive results, got %d", numGoroutines, aliveCount)
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(substr) > 0 && len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}