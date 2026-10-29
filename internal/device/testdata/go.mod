// This nested module keeps Linux sysfs fixtures (whose filenames contain ':')
// out of the parent Go module archive. Go module zip paths cannot contain ':'.
// The fixtures remain available when running Kepler's tests from a checkout.
module github.com/sustainable-computing-io/kepler/internal/device/testdata

go 1.27.0
