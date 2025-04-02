package device

import "fmt"

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
