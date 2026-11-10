//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package real

import (
	"math"
	"os"

	"golang.org/x/sys/unix"
)

// streamSpace measures the volume containing the already opened directory.
func streamSpace(parent *os.File) (int64, error) {
	var stat unix.Statfs_t
	if err := unix.Fstatfs(int(parent.Fd()), &stat); err != nil {
		return 0, err
	}
	if stat.Bavail <= 0 || stat.Bsize <= 0 {
		return 0, nil
	}
	block := uint64(stat.Bsize)
	if uint64(stat.Bavail) > math.MaxInt64/block {
		return math.MaxInt64, nil
	}
	return int64(uint64(stat.Bavail) * block), nil
}
