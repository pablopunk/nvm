//go:build darwin || linux

package real

import "golang.org/x/sys/unix"

type directoryAccess struct{}

func (directoryAccess) CanWrite(path string) bool {
	return unix.Faccessat(unix.AT_FDCWD, path, unix.W_OK|unix.X_OK, unix.AT_EACCESS) == nil
}
