//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package real

import (
	"golang.org/x/sys/unix"
	"math"
	"os"
)

func pictureTemporarySpace() (int64, error) {
	return spaceAt(os.TempDir())
}

func spaceAt(path string) (int64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	if stat.Bavail <= 0 || stat.Bsize <= 0 {
		return 0, nil
	}
	block := uint64(stat.Bsize)
	if block == 0 {
		return 0, errPictureStagingSpace
	}
	if uint64(stat.Bavail) > math.MaxInt64/block {
		return math.MaxInt64, nil
	}
	return int64(uint64(stat.Bavail) * block), nil
}
