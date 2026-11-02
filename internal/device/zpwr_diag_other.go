// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !(linux && s390x)

package device

import (
	"fmt"
	"runtime"
)

// openPIBSource is only implemented on linux/s390x (IBM Z / LinuxONE).
func openPIBSource(_ string) (pibSource, error) {
	return nil, fmt.Errorf("%w: zpwr requires linux/s390x (IBM Z or LinuxONE), running on %s/%s",
		errPIBNotSupported, runtime.GOOS, runtime.GOARCH)
}
