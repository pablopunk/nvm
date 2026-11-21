//go:build unix

package apps

import (
	"io/fs"
	"syscall"
)

// statIdentity reads the device, inode and owner that POSIX systems put in the stat record.
func statIdentity(info fs.FileInfo) (dev, ino uint64, uid int, ok bool) {
	st, isStat := info.Sys().(*syscall.Stat_t)
	if !isStat {
		return 0, 0, 0, false
	}
	return uint64(st.Dev), uint64(st.Ino), int(st.Uid), true
}
