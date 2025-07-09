// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package monitor

import (
	"container/heap"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/sustainable-computing-io/kepler/internal/device"
)

// Resource represents any resource type that can be tracked by energy consumption
type Resource interface {
	StringID() string
	ZoneUsage() ZoneUsageMap
}

// TerminatedResourceTracker tracks the top N highest energy consuming terminated resources
// using a priority queue (min-heap) for fast insertion operations.
//
// IMPORTANT: This tracker is designed specifically for terminated resources, which  should ..
// follow these properties:
// - Once terminated, a resource cannot be terminated again
// - Energy consumption of terminated resources is immutable (frozen at termination)
// - No updates or re-additions of the same resource will occur
//
// These constraints allow for optimizations like skipping duplicate checks
type TerminatedResourceTracker[T Resource] struct {
	logger             *slog.Logger
	heap               Heap[T]           // min-heap for efficient eviction of lowest energy items
	resources          map[string]T      // ID -> Resource for O(1) lookup
	targetZone         device.EnergyZone // zone to use for energy comparison
	maxSize            int               // maximum number of resources to track
	minEnergyThreshold Energy            // minimum energy threshold to track a resource
}

// Heap implements a min-heap of resources sorted by energy consumption
type Heap[T Resource] []HeapItem[T]

// HeapItem represents a resource with its energy value in the heap
type HeapItem[T Resource] struct {
	resource    T
	ID          string
	EnergyTotal Energy
}

// NewTerminatedResourceTracker creates a new tracker with the specified energy zone, capacity, and minimum energy threshold
func NewTerminatedResourceTracker[T Resource](zone device.EnergyZone, maxSize int, minEnergyThreshold Energy, logger *slog.Logger) *TerminatedResourceTracker[T] {
	h := Heap[T]{}
	heap.Init(&h)

	// get the resource typename (Process, Container) and add it to logger context
	var zero T
	resourceType := reflect.TypeOf(zero).Elem().Name()
	loggerWithType := logger.With("service", "terminated-resource-tracker", "resource", resourceType)

	return &TerminatedResourceTracker[T]{
		logger:             loggerWithType,
		heap:               h,
		resources:          make(map[string]T),
		targetZone:         zone,
		maxSize:            maxSize,
		minEnergyThreshold: minEnergyThreshold,
	}
}

// Add adds a terminated resource to the tracker.
//
// NOTE: OPTIMIZATION: Since terminated resources are immutable and
// cannot be re-terminated, this method assumes that:
// - The same resource ID will never be added twice in normal operation
// - No energy updates for existing resources will occur
// - Resources added here represent the final energy state at termination
//
// If the tracker is at capacity, the resource with the lowest energy will be evicted
// if the new resource has higher energy.
func (trt *TerminatedResourceTracker[T]) Add(resource T) {
	// If maxSize is 0, feature is disabled - don't track anything
	if trt.maxSize == 0 {
		return
	}

	// Check if already tracking this resource
	// NOTE: Since terminated resources are immutable and should never be re-added,
	// this check is for safety but should never trigger in normal operation
	id := resource.StringID()
	if _, exists := trt.resources[id]; exists {
		trt.logger.Warn("Resource already tracked in terminated resource tracker", "id", id)
		return // Ignore duplicate - terminated resource already tracked
	}

	// Get the energy from the target zone for this resource
	energyTotal := Energy(0)
	if zoneUsage, exists := resource.ZoneUsage()[trt.targetZone]; exists {
		energyTotal = zoneUsage.EnergyTotal
	}

	// Filter out resources that don't meet the minimum energy threshold
	if energyTotal < trt.minEnergyThreshold {
		trt.logger.Debug("Filtering out terminated resource with low energy",
			"id", id, "energy", energyTotal, "threshold", trt.minEnergyThreshold)
		return
	}

	trt.logger.Debug("Keeping track of terminated resource", "id", id, "energy", energyTotal)
	newItem := HeapItem[T]{
		ID:          id,
		EnergyTotal: energyTotal,
		resource:    resource,
	}

	// nothing more to do if there is room or maxSize is unlimited
	if len(trt.heap) < trt.maxSize || trt.maxSize < 0 {
		// Room available, just add
		heap.Push(&trt.heap, newItem)
		trt.resources[id] = resource
		return
	}

	// At capacity, check if new item has higher energy than minimum
	if len(trt.heap) > 0 && newItem.EnergyTotal > trt.heap[0].EnergyTotal {
		// Evict lowest energy resource
		minItem := heap.Pop(&trt.heap).(HeapItem[T])
		delete(trt.resources, minItem.ID)

		// Add new higher-energy resource
		heap.Push(&trt.heap, newItem)
		trt.resources[id] = resource
	}
}

// Items returns all tracked workloads as a map[string]T where the key is the resource ID
func (trt *TerminatedResourceTracker[T]) Items() map[string]T {
	// Return a copy of the map to prevent external modifications
	result := make(map[string]T, len(trt.resources))
	for id, resource := range trt.resources {
		result[id] = resource
	}
	return result
}

// Size returns the current number of tracked resources
func (trt *TerminatedResourceTracker[T]) Size() int {
	return len(trt.resources)
}

// MaxSize returns the maximum capacity of the tracker
func (trt *TerminatedResourceTracker[T]) MaxSize() int {
	return trt.maxSize
}

// Clear removes all tracked resources
func (trt *TerminatedResourceTracker[T]) Clear() {
	trt.resources = make(map[string]T)
	trt.heap = trt.heap[:0] // Clear the slice but keep the underlying array
	heap.Init(&trt.heap)    // Re-initialize the heap
}

// EnergyZone returns the energy zone used for prioritization
func (trt *TerminatedResourceTracker[T]) EnergyZone() device.EnergyZone {
	return trt.targetZone
}

// String returns a string representation for debugging
func (trt *TerminatedResourceTracker[T]) String() string {
	maxSizeStr := fmt.Sprintf("%d", trt.MaxSize())
	if trt.MaxSize() == -1 {
		maxSizeStr = "unlimited"
	} else if trt.MaxSize() == 0 {
		maxSizeStr = "disabled"
	}

	return fmt.Sprintf("TerminatedResourceTracker{size: %d/%s, zone: %s}",
		trt.Size(), maxSizeStr, trt.targetZone.Name())
}

// stdlib/Heap interface implementation for Heap

func (h Heap[T]) Len() int { return len(h) }

func (h Heap[T]) Less(i, j int) bool {
	// Min-heap: smallest energy value at the root
	return h[i].EnergyTotal < h[j].EnergyTotal
}

func (h Heap[T]) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *Heap[T]) Push(x any) {
	*h = append(*h, x.(HeapItem[T]))
}

func (h *Heap[T]) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}
