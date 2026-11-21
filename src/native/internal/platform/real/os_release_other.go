//go:build !linux && !darwin && !windows

package real

import "errors"

type osRelease struct{}

func newOSRelease() osRelease { return osRelease{} }

func (osRelease) KernelRelease() (string, error) { return "", errors.New("kernel release unavailable") }
