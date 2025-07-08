// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/sustainable-computing-io/kepler/internal/device"
	testingclock "k8s.io/utils/clock/testing"
)

// TestSnapshotThreadSafety tests that multiple goroutines can call Snapshot concurrently without races.
func TestSnapshotThreadSafety(t *testing.T) {
	fakeClock := testingclock.NewFakeClock(time.Now())
	fakeMeter, err := device.NewFakeCPUMeter(nil)
	require.NoError(t, err)
	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	monitor := NewPowerMonitor(
		fakeMeter,
		WithClock(fakeClock),
		WithMaxStaleness(200*time.Millisecond),
		WithResourceInformer(resourceInformer),
	)

	err = monitor.Init()
	require.NoError(t, err)

	numGoroutines := runtime.NumCPU() * 2
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	const iterations = 5

	// use atomic counter to count errors from different go routines
	var errCount atomic.Int32
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				snapshot, err := monitor.Snapshot()
				if err != nil {
					t.Logf("Error getting snapshot: %v", err)
					errCount.Add(1)
					continue
				}
				if snapshot == nil {
					t.Log("Snapshot is nil")
					errCount.Add(1)
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int32(0), errCount.Load(), "Some snapshots failed to be retrieved")
}

// TestFreshSnapshotCaching tests that fresh snapshots are cached and not recomputed.
func TestFreshSnapshotCaching(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")
	pkg.On("MaxEnergy").Return(Energy(1_000_000))

	var computationCount atomic.Int32
	pkg.On("Energy").Run(func(args mock.Arguments) {
		// count the number of energy calculations
		computationCount.Add(1)
	}).Return(Energy(100_000), nil)

	energyZones := []device.EnergyZone{pkg}
	mockMeter.On("Zones").Return(energyZones, nil)
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)

	fakeClock := testingclock.NewFakeClock(time.Now())
	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	monitor := NewPowerMonitor(
		mockMeter,
		WithClock(fakeClock),
		WithMaxStaleness(100*time.Millisecond),
		WithLogger(slog.Default()),
		WithResourceInformer(resourceInformer),
	)

	err := monitor.Init()
	require.NoError(t, err)

	// first call to snapshot  should result in a new computation
	snapshot1, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot1)
	initialComputation := computationCount.Load()
	assert.Equal(t, initialComputation, int32(1), "Initial computation should have occurred")

	// next immediate call should use the cached snapshot (no new computation)
	snapshot2, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot2)
	assert.Equal(t, initialComputation, computationCount.Load(), "No new computation should have occurred")

	//  snapshots must be equal but not the same object (clones)
	assert.Equal(t, snapshot1, snapshot2, "Snapshots should have equal values")
	assert.NotSame(t, snapshot1, snapshot2, "Snapshots should be different objects (clones)")

	// move time past staleness threshold
	fakeClock.Step(200 * time.Millisecond)

	snapshot3, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot3)
	assert.Equal(t, computationCount.Load(), initialComputation+1, "New computation should have occurred")

	time2 := snapshot2.Timestamp
	time3 := snapshot3.Timestamp
	assert.True(t, time3.After(time2), "New snapshot should have a newer timestamp")
	mockMeter.AssertExpectations(t)
	pkg.AssertExpectations(t)
}

// TestStaleSnapshotRefreshing tests that stale snapshots are properly refreshed.
func TestStaleSnapshotRefreshing(t *testing.T) {
	// repeat the above using fake cpu meter
	fakeClock := testingclock.NewFakeClock(time.Now())
	fakeMeter, err := device.NewFakeCPUMeter(nil)
	require.NoError(t, err)

	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	monitor := NewPowerMonitor(
		fakeMeter,
		WithClock(fakeClock),
		WithMaxStaleness(100*time.Millisecond),
		WithResourceInformer(resourceInformer),
	)

	err = monitor.Init()
	require.NoError(t, err)

	snapshot1, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot1)
	time1 := snapshot1.Timestamp

	snapshot2, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot2)
	time2 := snapshot2.Timestamp

	// Times should be the same (cached value)
	assert.Equal(t, time1, time2, "Timestamps should be equal for fresh snapshots")

	// Advance time past staleness threshold
	fakeClock.Step(200 * time.Millisecond)

	// Third call should compute a new snapshot
	snapshot3, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, snapshot3)
	time3 := snapshot3.Timestamp

	// Time should be newer
	assert.True(t, time3.After(time2), "New snapshot should have a newer timestamp")
}

// TestSingleflightSnapshot tests that concurrent requests for stale data
// result in only one computation.
func TestSingleflightSnapshot(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}
	// only needs Name and Energy & Max for computation
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")

	var energyCallCount atomic.Int32
	pkg.On("Energy").Run(func(args mock.Arguments) {
		// NOTE: a small delay to increase likelihood of concurrent access
		time.Sleep(20 * time.Millisecond)
		energyCallCount.Add(1)
	}).Return(Energy(100_000), nil)

	pkg.On("MaxEnergy").Return(Energy(1_000_000))

	energyZones := []device.EnergyZone{pkg}
	mockMeter.On("Zones").Return(energyZones, nil)
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)

	// Create a fake clock to control time
	fakeClock := testingclock.NewFakeClock(time.Now())

	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	// Set up the monitor with a short staleness threshold
	monitor := NewPowerMonitor(
		mockMeter,
		WithClock(fakeClock),
		WithMaxStaleness(50*time.Millisecond),
		WithResourceInformer(resourceInformer),
	)

	// Initialize the monitor
	err := monitor.Init()
	require.NoError(t, err)

	// Get initial snapshot
	_, err = monitor.Snapshot()
	require.NoError(t, err)

	// Record the initial computation count
	initialCount := energyCallCount.Load()

	// Make the snapshot stale
	fakeClock.Step(100 * time.Millisecond)

	// Test with multiple concurrent goroutines all requesting a snapshot
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	var snapshots []*Snapshot
	var mutex sync.Mutex

	for range numGoroutines {
		go func() {
			defer wg.Done()
			snapshot, err := monitor.Snapshot() // Snapshot() must be thread safe
			if err != nil {
				t.Logf("Error getting snapshot: %v", err)
				return
			}

			mutex.Lock()
			snapshots = append(snapshots, snapshot)
			mutex.Unlock()
		}()
	}

	wg.Wait()

	assert.Equal(t, numGoroutines, len(snapshots), "Each goroutine should receive a snapshot")

	// Verify snapshots are consistent
	for i := 1; i < len(snapshots); i++ {
		assert.Equal(t, snapshots[0].Timestamp, snapshots[i].Timestamp,
			"All snapshots should have the same timestamp")
	}

	// Check that only one computation was performed
	// (initial + 1 for the concurrent requests)
	assert.Equal(t, initialCount+1, energyCallCount.Load(),
		"Only one additional computation should have occurred")

	mockMeter.AssertExpectations(t)
	pkg.AssertExpectations(t)
}

// TestSnapshot_ComputeFailures tests how snapshot handles errors during computation
func TestSnapshot_ComputeFailures(t *testing.T) {
	mockMeter := &MockCPUPowerMeter{}

	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")
	pkg.On("Index").Return(0)

	// first call to Energy succeeds, second fails
	pkg.On("Energy").Return(Energy(100_000), nil).Once()
	pkg.On("Energy").Return(Energy(0), assert.AnError).Once()

	mockMeter.On("Zones").Return([]device.EnergyZone{pkg}, nil)
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)

	fakeClock := testingclock.NewFakeClock(time.Now())
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	monitor := NewPowerMonitor(
		mockMeter,
		WithLogger(logger),
		WithClock(fakeClock),
		WithMaxStaleness(100*time.Millisecond),
		WithResourceInformer(resourceInformer),
	)

	err := monitor.Init()
	require.NoError(t, err)

	// first call should succeed
	s1, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, s1)

	// make data stale
	fakeClock.Step(200 * time.Millisecond)

	// second call will call `ensureFreshness` will fail and should return error and nil
	s2, err := monitor.Snapshot()
	assert.Error(t, err, "Should return error when computation fails")
	assert.Nil(t, s2, "Should not return the previous snapshot on error")
	mockMeter.AssertExpectations(t)
	pkg.AssertExpectations(t)
}

// TestSnapshot_ConcurrentAfterError tests concurrent snapshot requests after a computation error
func TestSnapshot_ConcurrentAfterError(t *testing.T) {
	// Setup mocks
	mockMeter := &MockCPUPowerMeter{}

	// Mock zones
	pkg := &MockEnergyZone{}
	pkg.On("Name").Return("package")
	pkg.On("Index").Return(0)
	pkg.On("Path").Return("/sys/class/powercap/intel-rapl/intel-rapl:0")

	// First call succeeds, second fails, rest succeed again
	pkg.On("Energy").Return(Energy(100_000), nil).Once()
	pkg.On("Energy").Return(Energy(0), assert.AnError).Once()

	// after the error, all subsequent calls from different goroutines must succeed

	numGoroutines := runtime.NumCPU() * 3

	var energyCallCount atomic.Int32
	pkg.On("Energy").Run(func(args mock.Arguments) {
		// NOTE: a small delay to increase likelihood of concurrent access
		time.Sleep(20 * time.Millisecond)
		energyCallCount.Add(1)
	}).Return(Energy(200_000), nil).Times(numGoroutines)

	pkg.On("MaxEnergy").Return(Energy(1_000_000))

	mockMeter.On("Name").Return("mock-cpu")
	mockMeter.On("Init", mock.Anything).Return(nil)
	mockMeter.On("Zones").Return([]device.EnergyZone{pkg}, nil)
	mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)

	// Create the monitor
	fakeClock := testingclock.NewFakeClock(time.Now())
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	tr := CreateTestResources()
	resourceInformer := &MockResourceInformer{}
	resourceInformer.SetExpectations(t, tr)
	resourceInformer.On("Refresh").Return(nil)

	monitor := NewPowerMonitor(
		mockMeter,
		WithLogger(logger),
		WithClock(fakeClock),
		WithMaxStaleness(100*time.Millisecond),
		WithResourceInformer(resourceInformer),
	)

	// Initialize
	err := monitor.Init()
	require.NoError(t, err)

	// First call should succeed and create a snapshot
	s1, err := monitor.Snapshot()
	require.NoError(t, err)
	require.NotNil(t, s1)

	// Advance clock to make data stale
	fakeClock.Step(200 * time.Millisecond)

	// Second call will try to compute and fail, but should return the old data
	s2, err := monitor.Snapshot()
	assert.Error(t, err, "Should return error when computation fails")
	assert.Nil(t, s2, "Should return nil on error")

	// stale snapshot
	fakeClock.Step(200 * time.Millisecond)

	// make concurrent calls after the error
	var wg sync.WaitGroup
	type result struct {
		s   *Snapshot
		err error
	}
	results := make(chan result, numGoroutines)

	wg.Add(numGoroutines)
	for range numGoroutines {
		go func() {
			defer wg.Done()
			s, err := monitor.Snapshot()
			results <- result{s, err}
		}()
	}

	wg.Wait()
	close(results)

	// Validate
	successCount := 0
	var lastSnapshot *Snapshot

	for res := range results {
		if res.err == nil {
			successCount++
			lastSnapshot = res.s
		}
	}

	// All calls to snapshot should  succeeded
	assert.Equal(t, numGoroutines, successCount, "All concurrent calls should succeed")

	// and the computation should have happened exactly once
	assert.Equal(t, int32(1), energyCallCount.Load(),
		"Computation should happen exactly once despite concurrent calls")

	// Verify new data was used (timestamp should be different)
	assert.NotEqual(t, s1.Timestamp, lastSnapshot.Timestamp,
		"New snapshot should have a different timestamp")
}

func TestPowerMonitor_ConcurrentCollection(t *testing.T) {
	t.Run("snapshots while collection should not cause race conditions", func(t *testing.T) {
		mockMeter := &MockCPUPowerMeter{}

		// Set up pkg mock
		pkg := &MockEnergyZone{}
		pkg.On("Name").Return("package")
		pkg.On("MaxEnergy").Return(Energy(1000 * Joule))

		// Energy reads will return increasing values with artificial delay
		var energyVal atomic.Uint64
		energyVal.Store(1000)

		var computationCount atomic.Int32
		pkg.On("Energy").Run(func(args mock.Arguments) {
			computationCount.Add(1)
			// Add small delay to simulate slow I/O and increase chance of concurrent access
			time.Sleep(8 * time.Millisecond)
			// Increment by 10 each time
			energyVal.Add(10)
		}).Return(Energy(energyVal.Load()), nil).Maybe()

		mockMeter.On("Zones").Return([]EnergyZone{pkg}, nil)
		mockMeter.On("PrimaryEnergyZone").Return(pkg, nil)

		fakeClock := testingclock.NewFakeClock(time.Now())

		tr := CreateTestResources()
		resourceInformer := &MockResourceInformer{}
		resourceInformer.SetExpectations(t, tr)
		resourceInformer.On("Refresh").Return(nil)

		monitor := NewPowerMonitor(
			mockMeter,
			WithInterval(50*time.Millisecond),
			WithClock(fakeClock),
			WithMaxStaleness(30*time.Millisecond), // Short staleness to force recalculation
			WithResourceInformer(resourceInformer),
		)

		// Initialize monitor
		err := monitor.Init()
		require.NoError(t, err)

		// run in background
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			_ = monitor.Run(ctx)
		}()

		// wait for monitor to startt
		time.Sleep(10 * time.Millisecond)

		// request snapshots concurrently and  advance time
		var wg sync.WaitGroup
		numGoroutines := runtime.NumCPU() * 3
		const numIterations = 5

		// Track any errors
		var encounteredErr atomic.Bool

		for i := range numGoroutines {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := range numIterations {

					// Each goroutine requests a snapshot
					snapshot, err := monitor.Snapshot()
					if err != nil {
						t.Logf("Goroutine %d iteration %d: Error getting snapshot: %v", id, j, err)
						encounteredErr.Store(true)
						continue
					}

					// Verify snapshot is valid
					if snapshot == nil {
						t.Logf("Goroutine %d iteration %d: Got nil snapshot", id, j)
						encounteredErr.Store(true)
						continue
					}

					// Verify snapshot has zone data
					if _, ok := snapshot.Node.Zones[pkg]; !ok {
						t.Logf("Goroutine %d iteration %d: Missing zone data", id, j)
						encounteredErr.Store(true)
					}

					// sleep a bit to increase chance of concurrent access
					time.Sleep(time.Duration(id) * time.Millisecond)
				}
			}(i)
		}

		// keep clock ticking while snapshots are requested
		go func() {
			for range numIterations {
				fakeClock.Step(50 * time.Millisecond)
				time.Sleep(10 * time.Millisecond)
			}
		}()

		wg.Wait()

		// verify no errors were encountered
		assert.False(t, encounteredErr.Load(), "Some goroutines encountered errors")

		mockMeter.AssertExpectations(t)
		pkg.AssertExpectations(t)
	})

	t.Run("Snapshot handles stale data with concurrent requests", func(t *testing.T) {
		mockMeter := &MockCPUPowerMeter{}

		pkg := &MockEnergyZone{}
		pkg.On("Name").Return("package")

		// Track the number of collections by tracking energy reads
		var computeCount atomic.Int32
		pkg.On("Energy").Run(func(args mock.Arguments) {
			// Add delay to increase chance of concurrent access
			time.Sleep(10 * time.Millisecond)
			computeCount.Add(1)
		}).Return(Energy(100*Joule), nil).Maybe()

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
			WithMaxStaleness(50*time.Millisecond),
			WithResourceInformer(resourceInformer),
		)

		err := monitor.Init()
		require.NoError(t, err)

		// Init should not collect
		assert.Equal(t, int32(0), computeCount.Load())

		// Launch multiple concurrent requests for the stale snapshot
		var wg sync.WaitGroup
		numGoroutines := runtime.NumCPU() * 3

		for range numGoroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()

				snapshot, err := monitor.Snapshot()
				assert.NoError(t, err)
				assert.NotNil(t, snapshot)

				// Verify snapshot has zone data
				assert.Contains(t, snapshot.Node.Zones, pkg)
			}()
		}

		wg.Wait()

		// Despite having may concurrent requests for a stale snapshot,
		// we should have computed power only once due to singleflight
		assert.Equal(t, int32(1), computeCount.Load(),
			"Computation should happen exactly once despite concurrent requests")

		mockMeter.AssertExpectations(t)
		pkg.AssertExpectations(t)
	})
}
