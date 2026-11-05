//go:build darwin

package real

import "syscall"

type osRelease struct{}

func newOSRelease() osRelease { return osRelease{} }

func (osRelease) KernelRelease() (string, error) { return syscall.Sysctl("kern.osrelease") }
