// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
)

// Energy represents energy usage as an uint64 MicroJoule count.
// The maximum energy that can be captured is
// Use functions Joule and MicroJoule to get the energy value as
// Joule or MicroJoule
type Energy uint64

// Joule returns the underlying energy value as Joules
func (e Energy) Joules() float64 {
	return float64(e) / 1_000_000
}

func (e Energy) MicroJoules() uint64 {
	return uint64(e)
}

func (e Energy) String() string {
	return fmt.Sprintf("%fJ", e.Joules())
}

// Power represents power usage as an float64 MicroWatts.
// Use functions Watts and MicroWatts to get the power value as
// Watts or MicroWatts
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
	return fmt.Sprintf("%fW", p.Watts())
}
