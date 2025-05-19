// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
)

// Energy represents energy usage as an uint64 MicroJoule count.
// The maximum energy that can be captured is 2^64 - 1 MicroJoules
// Use functions Joules, MilliJoules and MicroJoules to get the energy
// value as Joule, MilliJoule or MicroJoule respectively
type Energy uint64

const (
	MicroJoule Energy = 1
	MilliJoule        = 1000 * MicroJoule
	Joule             = 1000 * MilliJoule
)

func (e Energy) MicroJoules() uint64 {
	return uint64(e)
}

func (e Energy) MilliJoules() float64 {
	return float64(e) / float64(MilliJoule)
}

func (e Energy) Joules() float64 {
	return float64(e) / float64(Joule)
}

func (e Energy) String() string {
	return fmt.Sprintf("%.2fJ", e.Joules())
}

// Power represents power usage as an float64 MicroWatts.
// Use functions Watts, MilliWatts and MicroWatts to get the power value as
// Watts, MilliWatts or MicroWatts respectively
type Power float64

const (
	MicroWatt Power = 1.0
	MilliWatt       = 1000 * MicroWatt
	Watt            = 1000 * MilliWatt
)

func (p Power) MicroWatts() float64 {
	return float64(p)
}

func (p Power) MilliWatts() float64 {
	return float64(p / MilliWatt)
}

func (p Power) Watts() float64 {
	return float64(p / Watt)
}

func (p Power) String() string {
	return fmt.Sprintf("%.2fW", p.Watts())
}
