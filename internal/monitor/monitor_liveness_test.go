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
	tolerance := 2.0 // Default tolerance
	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(interval), // Active mode
		WithHealthCheckTolerance(tolerance),
	)

	// Simulate stale heartbeat (older than tolerance*interval)
	staleTime := time.Now().Add(-time.Duration(float64(interval) * (tolerance + 1))) // Beyond tolerance
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
	if !containsString(err.Error(), "tolerance=2.0x interval") {
		t.Errorf("Expected tolerance info in error, got: %v", err)
	}
}

func TestPowerMonitor_IsLive_ActiveMode_HeartbeatAtLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	interval := 1 * time.Second
	tolerance := 2.0
	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(interval),
		WithHealthCheckTolerance(tolerance),
	)

	// Simulate heartbeat exactly at the tolerance limit
	toleranceDuration := time.Duration(float64(interval) * tolerance)
	limitTime := time.Now().Add(-toleranceDuration + 100*time.Millisecond) // Just under the limit
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

func TestPowerMonitor_IsLive_CustomTolerance(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	interval := 1 * time.Second
	customTolerance := 3.5 // 3.5x interval tolerance

	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(interval),
		WithHealthCheckTolerance(customTolerance),
	)

	ctx := context.Background()

	// Test heartbeat that would be stale with default tolerance (2.0) but fresh with custom (3.5)
	heartbeatTime := time.Now().Add(-time.Duration(float64(interval) * 3.0)) // 3x interval ago
	atomic.StoreInt64(&pm.lastCollectUnixNano, heartbeatTime.UnixNano())

	// Should be alive with custom tolerance
	alive, err := pm.IsLive(ctx)
	if err != nil {
		t.Errorf("Expected no error with custom tolerance, got: %v", err)
	}
	if !alive {
		t.Error("Expected alive=true with custom tolerance")
	}

	// Test heartbeat beyond custom tolerance
	staleTime := time.Now().Add(-time.Duration(float64(interval) * 4.0)) // 4x interval ago, beyond 3.5x
	atomic.StoreInt64(&pm.lastCollectUnixNano, staleTime.UnixNano())

	// Should not be alive beyond custom tolerance
	alive, err = pm.IsLive(ctx)
	if alive {
		t.Error("Expected alive=false beyond custom tolerance")
	}
	if err == nil {
		t.Error("Expected error beyond custom tolerance")
	}
	if !containsString(err.Error(), "tolerance=3.5x interval") {
		t.Errorf("Expected custom tolerance info in error, got: %v", err)
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