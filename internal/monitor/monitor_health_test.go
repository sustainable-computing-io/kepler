// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/sustainable-computing-io/kepler/internal/device"
	testingclock "k8s.io/utils/clock/testing"
)

func TestPowerMonitor_IsLive_NotInitialized(t *testing.T) {
	meter, err := device.NewFakeCPUMeter([]string{"pkg"})
	assert.NoError(t, err)
	pm := NewPowerMonitor(meter)

	// Before initialization, should not be live
	assert.False(t, pm.IsLive())
}

func TestPowerMonitor_IsLive_Initialized(t *testing.T) {
	meter, err := device.NewFakeCPUMeter([]string{"pkg"})
	assert.NoError(t, err)
	pm := NewPowerMonitor(meter)

	err = pm.Init()
	assert.NoError(t, err)

	// After initialization, should be live
	assert.True(t, pm.IsLive())
}

func TestPowerMonitor_IsLive_FatalError(t *testing.T) {
	meter, err := device.NewFakeCPUMeter([]string{"pkg"})
	assert.NoError(t, err)
	pm := NewPowerMonitor(meter)

	err = pm.Init()
	assert.NoError(t, err)

	// Simulate fatal error
	pm.fatalError.Store(true)

	// Should not be live when fatal error occurred
	assert.False(t, pm.IsLive())
}

func TestPowerMonitor_IsReady_NotInitialized(t *testing.T) {
	meter, err := device.NewFakeCPUMeter([]string{"pkg"})
	assert.NoError(t, err)
	pm := NewPowerMonitor(meter)

	// Before initialization, should not be ready
	assert.False(t, pm.IsReady())
}

func TestPowerMonitor_IsReady_InitializedNoSnapshot(t *testing.T) {
	meter, err := device.NewFakeCPUMeter([]string{"pkg"})
	assert.NoError(t, err)
	pm := NewPowerMonitor(meter)

	err = pm.Init()
	assert.NoError(t, err)

	// After initialization but before first snapshot, should not be ready
	assert.False(t, pm.IsReady())
}

func TestPowerMonitor_IsReady_WithFreshSnapshot(t *testing.T) {
	meter, err := device.NewFakeCPUMeter([]string{"pkg"})
	assert.NoError(t, err)
	fakeClock := testingclock.NewFakeClock(time.Now())

	pm := NewPowerMonitor(meter,
		WithClock(fakeClock),
		WithMaxStaleness(10*time.Second),
	)

	initErr := pm.Init()
	assert.NoError(t, initErr)

	// Create a snapshot
	snapshot := NewSnapshot()
	snapshot.Timestamp = fakeClock.Now()
	pm.snapshot.Store(snapshot)

	// Should be ready with fresh snapshot
	assert.True(t, pm.IsReady())
}

func TestPowerMonitor_IsReady_WithStaleSnapshot(t *testing.T) {
	meter, err := device.NewFakeCPUMeter([]string{"pkg"})
	assert.NoError(t, err)
	fakeClock := testingclock.NewFakeClock(time.Now())

	pm := NewPowerMonitor(meter,
		WithClock(fakeClock),
		WithMaxStaleness(10*time.Second),
	)

	err = pm.Init()
	assert.NoError(t, err)

	// Create a snapshot in the past
	snapshot := NewSnapshot()
	snapshot.Timestamp = fakeClock.Now()
	pm.snapshot.Store(snapshot)

	// Advance clock beyond staleness threshold
	fakeClock.Step(15 * time.Second)

	// Should not be ready with stale snapshot
	assert.False(t, pm.IsReady())
}

func TestPowerMonitor_IsReady_NotLive(t *testing.T) {
	meter, err := device.NewFakeCPUMeter([]string{"pkg"})
	assert.NoError(t, err)
	fakeClock := testingclock.NewFakeClock(time.Now())

	pm := NewPowerMonitor(meter,
		WithClock(fakeClock),
		WithMaxStaleness(10*time.Second),
	)

	err = pm.Init()
	assert.NoError(t, err)

	// Create a fresh snapshot
	snapshot := NewSnapshot()
	snapshot.Timestamp = fakeClock.Now()
	pm.snapshot.Store(snapshot)

	// Simulate fatal error (not live)
	pm.fatalError.Store(true)

	// Should not be ready if not live, even with fresh snapshot
	assert.False(t, pm.IsReady())
}

func TestPowerMonitor_IsReady_ZeroTimestamp(t *testing.T) {
	meter, err := device.NewFakeCPUMeter([]string{"pkg"})
	assert.NoError(t, err)
	fakeClock := testingclock.NewFakeClock(time.Now())

	pm := NewPowerMonitor(meter,
		WithClock(fakeClock),
		WithMaxStaleness(10*time.Second),
	)

	err = pm.Init()
	assert.NoError(t, err)

	// Create a snapshot with zero timestamp
	snapshot := NewSnapshot()
	snapshot.Timestamp = time.Time{} // Zero value
	pm.snapshot.Store(snapshot)

	// Should not be ready with zero timestamp
	assert.False(t, pm.IsReady())
}

func TestPowerMonitor_HealthCheck_Integration(t *testing.T) {
	meter, err := device.NewFakeCPUMeter([]string{"pkg"})
	assert.NoError(t, err)
	fakeClock := testingclock.NewFakeClock(time.Now())

	pm := NewPowerMonitor(meter,
		WithClock(fakeClock),
		WithMaxStaleness(10*time.Second),
		WithInterval(0), // No automatic collection
	)

	// Initially not live or ready
	assert.False(t, pm.IsLive())
	assert.False(t, pm.IsReady())

	// After init, live but not ready
	initErr := pm.Init()
	assert.NoError(t, initErr)
	assert.True(t, pm.IsLive())
	assert.False(t, pm.IsReady())

	// After first snapshot, both live and ready
	snapshot := NewSnapshot()
	snapshot.Timestamp = fakeClock.Now()
	pm.snapshot.Store(snapshot)
	assert.True(t, pm.IsLive())
	assert.True(t, pm.IsReady())

	// After data becomes stale, live but not ready
	fakeClock.Step(15 * time.Second)
	assert.True(t, pm.IsLive())
	assert.False(t, pm.IsReady())

	// After fatal error, neither live nor ready
	pm.fatalError.Store(true)
	assert.False(t, pm.IsLive())
	assert.False(t, pm.IsReady())
}
