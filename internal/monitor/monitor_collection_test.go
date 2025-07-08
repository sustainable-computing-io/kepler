// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	testingclock "k8s.io/utils/clock/testing"
)

// TestCollectionLoop tests the initial collection when collectionLoop starts
func TestCollectionLoop(t *testing.T) {
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")
	pkg.On("Energy").Return(Energy(100*Joule), nil)
	// NOTE:  max energy is not called for the initial collection, since there is no diff calculation

	mockMeter := &MockCPUPowerMeter{}
	mockMeter.On("Zones").Return([]EnergyZone{pkg}, nil)
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)
	fakeClock := testingclock.NewFakeClock(time.Now())

	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	monitor := NewPowerMonitor(
		mockMeter,
		WithClock(fakeClock),
		WithInterval(0),
		WithResourceInformer(resourceInformer),
	)

	err := monitor.Init()
	require.NoError(t, err)

	dataCh := monitor.DataChannel()
	assertDataUpdated(t, dataCh, 3*time.Millisecond, "Initial collection should happen immediately")

	go monitor.collectionLoop()
	assertDataUpdated(t, dataCh, 10*time.Millisecond, "collectionLoop should run a collection as soon as it starts")

	// verify snapshot has been updated
	initialSnapshot := monitor.snapshot.Load()
	require.NotNil(t, initialSnapshot)
	initialTimestamp := initialSnapshot.Timestamp

	time.Sleep(50 * time.Millisecond)
	currentSnapshot := monitor.snapshot.Load()
	assert.Equal(t, initialTimestamp, currentSnapshot.Timestamp, "No further collections should happen with interval=0")

	mockMeter.AssertExpectations(t)
	pkg.AssertExpectations(t)
	pkg.AssertNotCalled(t, "MaxEnergy")
}

func TestPeriodicCollection(t *testing.T) {
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")

	var energyCalls atomic.Int32
	pkg.On("Energy").Run(func(args mock.Arguments) {
		energyCalls.Add(1)
	}).Return(Energy(100*Joule), nil)

	pkg.On("MaxEnergy").Return(Energy(1000 * Joule))

	mockMeter := &MockCPUPowerMeter{}
	mockMeter.On("Zones").Return([]EnergyZone{pkg}, nil)
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)
	fakeClock := testingclock.NewFakeClock(time.Now())

	resourceInformer := &MockResourceInformer{}
	resourceInformer.On("Refresh").Return(nil)

	tr := CreateTestResources()
	resourceInformer.SetExpectations(t, tr)

	collectionInterval := 50 * time.Millisecond
	monitor := NewPowerMonitor(
		mockMeter,
		WithResourceInformer(resourceInformer),
		WithClock(fakeClock),
		WithInterval(collectionInterval),
		WithMaxStaleness(collectionInterval/4),
	)

	dataCh := monitor.DataChannel()
	err := monitor.Init()
	require.NoError(t, err)
	assertDataUpdated(t, dataCh, 1*time.Millisecond, "expected data to be updated immediately after init")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		t.Log("Running collection")
		// ensure that the data channel is drained before we start
		assertDataChannelEmpty(t, dataCh, 1*time.Millisecond)
		err := monitor.Run(ctx)
		assert.NoError(t, err)
	}()

	t.Log("Waiting for first collection")
	assertDataUpdated(t, dataCh, 5*time.Millisecond, "expected first collection as soon as run is invoked")
	assert.GreaterOrEqual(t, energyCalls.Load(), int32(1), "Should have made at least one energy call")
	assertDataChannelEmpty(t, dataCh, 3*time.Millisecond)

	t.Log("Waiting for second collection")
	fakeClock.Step(collectionInterval)
	assertDataUpdated(t, dataCh, 10*time.Millisecond, "expected second collection after interval")
	assert.GreaterOrEqual(t, energyCalls.Load(), int32(2), "Should have made at least 2 energy call")
	assertDataChannelEmpty(t, dataCh, 3*time.Millisecond)

	t.Log("Waiting for third collection")
	fakeClock.Step(collectionInterval)
	assertDataUpdated(t, dataCh, 10*time.Millisecond, "expected second collection after interval")
	assert.GreaterOrEqual(t, energyCalls.Load(), int32(3), "Should have made at least 3 energy call")

	mockMeter.AssertExpectations(t)
	pkg.AssertExpectations(t)
}

func TestCollectionCancellation(t *testing.T) {
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")
	var energyCalls atomic.Int32
	pkg.On("Energy").Run(func(args mock.Arguments) {
		energyCalls.Add(1)
	}).Return(Energy(100*Joule), nil)

	pkg.On("MaxEnergy").Return(Energy(1000 * Joule))

	mockMeter := &MockCPUPowerMeter{}
	mockMeter.On("Zones").Return([]EnergyZone{pkg}, nil)
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)

	resourceInformer := &MockResourceInformer{}

	tr := CreateTestResources()
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	interval := 10 * time.Millisecond
	monitor := NewPowerMonitor(
		mockMeter,
		WithResourceInformer(resourceInformer),
		WithInterval(interval),
		WithMaxStaleness(interval/4),
	)

	err := monitor.Init()
	require.NoError(t, err)
	assertDataUpdated(t, monitor.DataChannel(), 1*time.Millisecond, "expected data to be updated immediately after init")

	initialSnapshot := monitor.snapshot.Load()
	require.Nil(t, initialSnapshot) // no snapshot yet

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	runCh := make(chan struct{})
	go func() {
		// ensure that the data channel is drained before we start
		assertDataChannelEmpty(t, monitor.DataChannel(), 1*time.Millisecond)
		err := monitor.Run(ctx)
		assert.NoError(t, err)
		close(runCh)
	}()

	assertDataUpdated(t, monitor.DataChannel(), 5*time.Millisecond, "expected first collection as soon as run is invoked")

	// ensure that the first collection has been made
	snapshotAfterFirstCollection := monitor.snapshot.Load()
	require.NotNil(t, snapshotAfterFirstCollection)

	waitInterval := 50 * time.Millisecond
	time.Sleep(waitInterval)
	cancel()

	select {
	case <-runCh:
		t.Log("Run exited as expected")

	case <-time.After(100 * time.Millisecond):
		t.Fatal("Run didn't exit after context cancellation")
	}
	assert.GreaterOrEqual(t, energyCalls.Load(), int32(waitInterval/interval)-1, "Should have made at least 5 energy call")

	snapshotAfterRun := monitor.snapshot.Load()
	require.NotNil(t, snapshotAfterRun)

	// ensure that no more collections happen
	time.Sleep(3 * interval)

	currentSnapshot := monitor.snapshot.Load()
	assert.Equal(t, snapshotAfterRun.Timestamp, currentSnapshot.Timestamp,
		"No collections should happen after cancellation")

	mockMeter.AssertExpectations(t)
	pkg.AssertExpectations(t)
}

func TestScheduleNextCollection(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")

	var collectionCount atomic.Int32
	pkg.On("Energy").Run(func(args mock.Arguments) {
		collectionCount.Add(1)
	}).Return(Energy(100*Joule), nil).Maybe()
	pkg.On("MaxEnergy").Return(Energy(1000 * Joule))

	mockMeter.On("Zones").Return([]EnergyZone{pkg}, nil)
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)

	fakeClock := testingclock.NewFakeClock(time.Now())

	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)

	resourceInformer.On("Refresh").Return(nil)

	collectionInterval := 50 * time.Millisecond
	monitor := NewPowerMonitor(
		mockMeter,
		WithClock(fakeClock),
		WithResourceInformer(resourceInformer),
		WithInterval(collectionInterval),
		WithMaxStaleness(collectionInterval/4),
	)

	err := monitor.Init()
	require.NoError(t, err)
	assertDataUpdated(t, monitor.DataChannel(), 1*time.Millisecond, "expected data to be updated immediately after init")

	monitor.scheduleNextCollection()
	assert.Equal(t, int32(0), collectionCount.Load())
	fakeClock.Step(collectionInterval)

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int32(1), collectionCount.Load(), "Collection should happen after interval")

	fakeClock.Step(collectionInterval)
	time.Sleep(20 * time.Millisecond)

	assert.Equal(t, int32(2), collectionCount.Load(), "Second collection should happen")

	monitor.collectionCancel()

	fakeClock.Step(collectionInterval)
	time.Sleep(20 * time.Millisecond)

	finalCount := collectionCount.Load()
	assert.Equal(t, int32(2), finalCount, "No more collections after cancellation")

	mockMeter.AssertExpectations(t)
	pkg.AssertExpectations(t)
}

func TestCollectionWithDataSignaling(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")
	pkg.On("Energy").Return(Energy(100*Joule), nil).Twice()
	pkg.On("MaxEnergy").Return(Energy(1000*Joule), nil).Once()

	mockMeter.On("Zones").Return([]EnergyZone{pkg}, nil)
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)

	fakeClock := testingclock.NewFakeClock(time.Now())
	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	interval := 20 * time.Millisecond
	monitor := NewPowerMonitor(
		mockMeter,
		WithClock(fakeClock),
		WithInterval(interval),
		WithMaxStaleness(1*time.Millisecond),
		WithResourceInformer(resourceInformer),
	)

	dataCh := monitor.DataChannel()
	err := monitor.Init()
	require.NoError(t, err)
	assertDataUpdated(t, monitor.DataChannel(), 1*time.Millisecond, "expected data to be updated immediately after init")

	go monitor.collectionLoop()
	time.Sleep(3 * time.Millisecond) // wait for collectionLoop to start

	fakeClock.Step(interval)
	assertDataUpdated(t, dataCh, interval, "expected data to be updated immediately after collection")

	fakeClock.Step(interval)
	assertDataUpdated(t, dataCh, interval, "expected data to be updated immediately after collection")

	monitor.collectionCancel()

	mockMeter.AssertExpectations(t)
	pkg.AssertExpectations(t)
}

func TestCollectionErrorHandling(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")

	pkg.On("Energy").Return(Energy(100*Joule), nil).Once()
	// Error on second read
	pkg.On("Energy").Return(Energy(0), assert.AnError).Once()
	pkg.On("Index").Return(0) // is used for logging

	pkg.On("Energy").Return(Energy(200*Joule), nil).Maybe()

	pkg.On("MaxEnergy").Return(Energy(1000 * Joule))

	mockMeter.On("Zones").Return([]EnergyZone{pkg}, nil)
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)

	fakeClock := testingclock.NewFakeClock(time.Now())
	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	interval := 20 * time.Millisecond
	monitor := NewPowerMonitor(
		mockMeter,
		WithClock(fakeClock),
		WithInterval(interval),
		WithMaxStaleness(1*time.Millisecond),
		WithResourceInformer(resourceInformer),
	)

	err := monitor.Init()
	require.NoError(t, err)

	dataCh := monitor.DataChannel()
	assertDataUpdated(t, dataCh, 1*time.Millisecond, "expected data to be updated immediately after init")

	go monitor.collectionLoop()
	time.Sleep(3 * time.Millisecond) // wait for collectionLoop to start

	assertDataUpdated(t, dataCh, 20*time.Millisecond, "expected data to be updated immediately after collection")
	snapshot1, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot1)

	// Second collection should error and
	fakeClock.Step(20 * time.Millisecond)
	_, err = monitor.Snapshot()

	assert.Error(t, err, "Snapshot should return error after collection failure")

	fakeClock.Step(20 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	snapshot3, err := monitor.Snapshot()
	assert.NoError(t, err)
	assert.NotNil(t, snapshot3)

	assert.True(t, snapshot3.Timestamp.After(snapshot1.Timestamp))

	monitor.collectionCancel()

	mockMeter.AssertExpectations(t)
	pkg.AssertExpectations(t)
}

func assertDataChannelEmpty(t *testing.T, dataCh <-chan struct{}, timeout time.Duration) {
	t.Helper()
	select {
	case <-dataCh:
		t.Fatal("Data channel is not empty")
	case <-time.After(timeout):
		t.Log("No signal received as expected within", timeout)
	}
}

func assertDataUpdated(t *testing.T, dataCh <-chan struct{}, timeout time.Duration, msg string) {
	t.Helper()
	started := time.Now()
	select {
	case <-dataCh:
		t.Log("Received data updated signal as expected within", time.Since(started))
	case <-time.After(timeout):
		t.Fatalf("No signal received within %s; %s", timeout, msg)
	}
}
