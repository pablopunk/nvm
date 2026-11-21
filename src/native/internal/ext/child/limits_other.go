//go:build !linux

package child

import (
	"errors"
	"runtime"
)

// errMemoryLimitUnsupported is reported on stderr. Without a child address-space cap, the host's resident
// memory watch is the only memory bound on this OS.
var errMemoryLimitUnsupported = errors.New("address-space limits are not enforced on " + runtime.GOOS)

func applyMemoryLimit(uint64) error {
	return errMemoryLimitUnsupported
}
