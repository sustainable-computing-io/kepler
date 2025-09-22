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
)

func TestPowerMonitor_HealthCheck_CollectorFailure(t *testing.T) {
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

	ctx := context.Background()

	// Test 1: Initial state - not alive without heartbeat
	alive, err := pm.IsLive(ctx)
	if alive {
		t.Error("Expected not alive initially without heartbeat")
	}
	if err == nil || err.Error() != "no collection heartbeat yet" {
		t.Errorf("Expected 'no collection heartbeat yet' error, got: %v", err)
	}

	// Test 2: Simulate collector starting - set fresh heartbeat
	now := time.Now()
	atomic.StoreInt64(&pm.lastCollectUnixNano, now.UnixNano())

	alive, err = pm.IsLive(ctx)
	if !alive || err != nil {
		t.Errorf("Expected alive with fresh heartbeat, got alive=%v, err=%v", alive, err)
	}

	// Test 3: Simulate collector failure - heartbeat goes stale
	staleTime := time.Now().Add(-3 * interval) // 3x interval = stale
	atomic.StoreInt64(&pm.lastCollectUnixNano, staleTime.UnixNano())

	alive, err = pm.IsLive(ctx)
	if alive {
		t.Error("Expected not alive with stale heartbeat")
	}
	if err == nil || !containsStringIntegration(err.Error(), "collector stalled") {
		t.Errorf("Expected 'collector stalled' error, got: %v", err)
	}

	// Test 4: Simulate collector recovery - fresh heartbeat again
	freshTime := time.Now()
	atomic.StoreInt64(&pm.lastCollectUnixNano, freshTime.UnixNano())

	alive, err = pm.IsLive(ctx)
	if !alive || err != nil {
		t.Errorf("Expected alive after recovery, got alive=%v, err=%v", alive, err)
	}
}

func TestPowerMonitor_HealthCheck_StateTransitions(t *testing.T) {
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

	// State 1: Not Ready (no data)
	ready, err := pm.IsReady(ctx)
	if ready {
		t.Error("Expected not ready initially")
	}
	if err == nil || err.Error() != "no data yet" {
		t.Errorf("Expected 'no data yet' error, got: %v", err)
	}

	// State 2: Transition to Ready (add snapshot)
	snapshot := NewSnapshot()
	snapshot.Timestamp = time.Now()
	pm.snapshot.Store(snapshot)

	ready, err = pm.IsReady(ctx)
	if !ready || err != nil {
		t.Errorf("Expected ready with snapshot, got ready=%v, err=%v", ready, err)
	}

	// State 3: Transition back to Not Ready (remove snapshot)
	pm.snapshot.Store(nil)

	ready, err = pm.IsReady(ctx)
	if ready {
		t.Error("Expected not ready after removing snapshot")
	}
	if err == nil || err.Error() != "no data yet" {
		t.Errorf("Expected 'no data yet' error, got: %v", err)
	}

	// State 4: Transition to Ready again (restore snapshot)
	snapshot2 := NewSnapshot()
	snapshot2.Timestamp = time.Now()
	pm.snapshot.Store(snapshot2)

	ready, err = pm.IsReady(ctx)
	if !ready || err != nil {
		t.Errorf("Expected ready again with new snapshot, got ready=%v, err=%v", ready, err)
	}
}

func TestPowerMonitor_HealthCheck_PassiveActiveTransition(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	ctx := context.Background()

	// Test Passive Mode (interval = 0)
	pmPassive := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(0), // Passive mode
	)

	// Passive mode should always be alive and ready
	alive, err := pmPassive.IsLive(ctx)
	if !alive || err != nil {
		t.Errorf("Expected passive mode to be alive, got alive=%v, err=%v", alive, err)
	}

	ready, err := pmPassive.IsReady(ctx)
	if !ready || err != nil {
		t.Errorf("Expected passive mode to be ready, got ready=%v, err=%v", ready, err)
	}

	// Test Active Mode (interval > 0)
	pmActive := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(1*time.Second), // Active mode
	)

	// Active mode without heartbeat should not be alive
	alive, err = pmActive.IsLive(ctx)
	if alive {
		t.Error("Expected active mode without heartbeat to not be alive")
	}

	// Active mode without data should not be ready
	ready, err = pmActive.IsReady(ctx)
	if ready {
		t.Error("Expected active mode without data to not be ready")
	}
}

func TestPowerMonitor_HealthCheck_HeartbeatEdgeCases(t *testing.T) {
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

	ctx := context.Background()

	testCases := []struct {
		name           string
		ageOffset      time.Duration
		expectedAlive  bool
		expectError    bool
	}{
		{
			name:          "Fresh heartbeat",
			ageOffset:     -100 * time.Millisecond,
			expectedAlive: true,
			expectError:   false,
		},
		{
			name:          "At tolerance limit (just under 2x interval)",
			ageOffset:     -2*interval + 50*time.Millisecond,
			expectedAlive: true,
			expectError:   false,
		},
		{
			name:          "Just over tolerance limit",
			ageOffset:     -2*interval - 50*time.Millisecond,
			expectedAlive: false,
			expectError:   true,
		},
		{
			name:          "Very stale heartbeat",
			ageOffset:     -10 * interval,
			expectedAlive: false,
			expectError:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set heartbeat with specific age
			heartbeatTime := time.Now().Add(tc.ageOffset)
			atomic.StoreInt64(&pm.lastCollectUnixNano, heartbeatTime.UnixNano())

			alive, err := pm.IsLive(ctx)

			if alive != tc.expectedAlive {
				t.Errorf("Expected alive=%v, got %v", tc.expectedAlive, alive)
			}

			if tc.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestPowerMonitor_HealthCheck_ConcurrentStateChanges(t *testing.T) {
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

	// Set initial state
	snapshot := NewSnapshot()
	snapshot.Timestamp = time.Now()
	pm.snapshot.Store(snapshot)
	atomic.StoreInt64(&pm.lastCollectUnixNano, time.Now().UnixNano())

	// Run concurrent health checks while changing state
	const numCheckers = 50
	const numStateChanges = 10

	done := make(chan bool, numCheckers+numStateChanges)
	
	// Start health checkers
	for i := 0; i < numCheckers; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < 20; j++ {
				pm.IsLive(ctx)
				pm.IsReady(ctx)
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	// Start state changers
	for i := 0; i < numStateChanges; i++ {
		go func(id int) {
			defer func() { done <- true }()
			for j := 0; j < 10; j++ {
				if j%2 == 0 {
					// Set good state
					snap := NewSnapshot()
					snap.Timestamp = time.Now()
					pm.snapshot.Store(snap)
					atomic.StoreInt64(&pm.lastCollectUnixNano, time.Now().UnixNano())
				} else {
					// Set bad state
					pm.snapshot.Store(nil)
					atomic.StoreInt64(&pm.lastCollectUnixNano, 0)
				}
				time.Sleep(2 * time.Millisecond)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numCheckers+numStateChanges; i++ {
		<-done
	}

	// No panics or deadlocks = success
	t.Log("Concurrent state changes test completed successfully")
}

func TestPowerMonitor_HealthCheck_RealTimeProgression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-time test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	fakeMeter, err := device.NewFakeCPUMeter([]string{"package-0"}, device.WithFakeLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create fake CPU meter: %v", err)
	}

	interval := 500 * time.Millisecond
	pm := NewPowerMonitor(
		fakeMeter,
		WithLogger(logger),
		WithInterval(interval),
	)

	ctx := context.Background()

	// Set initial heartbeat
	atomic.StoreInt64(&pm.lastCollectUnixNano, time.Now().UnixNano())

	// Test progression over time
	times := []time.Duration{
		0,                           // Fresh
		interval,                    // 1x interval - still alive
		interval + 500*time.Millisecond, // 1.5x interval - still alive
		2*interval + 100*time.Millisecond, // Just over 2x interval - should be stale
	}

	for i, sleepDuration := range times {
		if i > 0 {
			time.Sleep(sleepDuration - times[i-1])
		}

		alive, err := pm.IsLive(ctx)
		expectedAlive := sleepDuration < 2*interval

		t.Logf("At %v (%.1fx interval): alive=%v, err=%v", 
			sleepDuration, float64(sleepDuration)/float64(interval), alive, err)

		if alive != expectedAlive {
			t.Errorf("At %v: expected alive=%v, got %v", sleepDuration, expectedAlive, alive)
		}
	}
}

// Helper function for string containment check (local to avoid conflicts)
func containsStringIntegration(s, substr string) bool {
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