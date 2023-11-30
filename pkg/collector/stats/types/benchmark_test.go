/*
Copyright 2023.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package types_test

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/sustainable-computing-io/kepler/pkg/collector/stats/types"
)

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

// SetAggrStat
func BenchmarkUInt64StatCollectionSetAggrStatMissed(b *testing.B) {
	instance := types.UInt64StatCollection{
		Stat: make(map[string]*types.UInt64Stat),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i)
		instance.SetAggrStat(key, 1)
		instance.SetAggrStat(key, 2)
	}
	b.StopTimer()
}

func BenchmarkUInt64StatCollectionSetAggrStatBothRandom(b *testing.B) {
	instance := types.UInt64StatCollection{
		Stat: make(map[string]*types.UInt64Stat),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i)
		instance.SetAggrStat(key, uint64(seededRand.Intn(10)+1))
		instance.SetAggrStat(key, uint64(seededRand.Intn(100)+1))
	}
	b.StopTimer()
}

func BenchmarkUInt64StatCollectionSetAggrStatCachedRandomNumber(b *testing.B) {
	instance := types.UInt64StatCollection{
		Stat: make(map[string]*types.UInt64Stat),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance.SetAggrStat("cache", uint64(seededRand.Intn(100)+1))
	}
	b.StopTimer()
}

func BenchmarkUInt64StatCollectionSetAggrStatCached(b *testing.B) {
	instance := types.UInt64StatCollection{
		Stat: make(map[string]*types.UInt64Stat),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance.SetAggrStat("cache", 1)
	}
	b.StopTimer()
}

// AddDeltaStat
func BenchmarkUInt64StatCollectionAddDeltaStatMissed(b *testing.B) {
	instance := types.UInt64StatCollection{
		Stat: make(map[string]*types.UInt64Stat),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i)
		instance.AddDeltaStat(key, 1)
		instance.AddDeltaStat(key, 2)
	}
	b.StopTimer()
}

func BenchmarkUInt64StatCollectionAddDeltaStatBothRandom(b *testing.B) {
	instance := types.UInt64StatCollection{
		Stat: make(map[string]*types.UInt64Stat),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i)
		instance.AddDeltaStat(key, uint64(seededRand.Intn(10)+1))
		instance.AddDeltaStat(key, uint64(seededRand.Intn(100)+1))
	}
	b.StopTimer()
}

func BenchmarkUInt64StatCollectionAddDeltaStatCachedRandomNumber(b *testing.B) {
	instance := types.UInt64StatCollection{
		Stat: make(map[string]*types.UInt64Stat),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance.AddDeltaStat("cache", uint64(seededRand.Intn(100)+1))
	}
	b.StopTimer()
}

func BenchmarkUInt64StatCollectionAddDeltaStatCached(b *testing.B) {
	instance := types.UInt64StatCollection{
		Stat: make(map[string]*types.UInt64Stat),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance.AddDeltaStat("cache", 1)
	}
	b.StopTimer()
}

// SetDeltaStat
func BenchmarkUInt64StatCollectionSetDeltaStatMissed(b *testing.B) {
	instance := types.UInt64StatCollection{
		Stat: make(map[string]*types.UInt64Stat),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i)
		instance.SetDeltaStat(key, 1)
		instance.SetDeltaStat(key, 2)
	}
	b.StopTimer()
}

func BenchmarkUInt64StatCollectionSetDeltaStatBothRandom(b *testing.B) {
	instance := types.UInt64StatCollection{
		Stat: make(map[string]*types.UInt64Stat),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i)
		instance.SetDeltaStat(key, uint64(seededRand.Intn(10)+1))
		instance.SetDeltaStat(key, uint64(seededRand.Intn(100)+1))
	}
	b.StopTimer()
}

func BenchmarkUInt64StatCollectionSetDeltaStatCachedRandomNumber(b *testing.B) {
	instance := types.UInt64StatCollection{
		Stat: make(map[string]*types.UInt64Stat),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance.SetDeltaStat("cache", uint64(seededRand.Intn(100)+1))
	}
	b.StopTimer()
}

func BenchmarkUInt64StatCollectionSetDeltaStatCached(b *testing.B) {
	instance := types.UInt64StatCollection{
		Stat: make(map[string]*types.UInt64Stat),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance.SetDeltaStat("cache", 1)
	}
	b.StopTimer()
}
