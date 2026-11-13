//go:build unix

package instance

import (
	"os"
	"syscall"
)

// ownedPrivately reports whether uid owns the folder and nobody else may enter it.
func ownedPrivately(info os.FileInfo, uid int) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == uid && info.Mode().Perm()&0o077 == 0
}
