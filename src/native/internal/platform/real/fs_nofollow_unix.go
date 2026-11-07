//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package real

import (
	"errors"
	"io"
	"os"

	"github.com/pablopunk/nvm/native/internal/platform"
	"golang.org/x/sys/unix"
)

// ReadFileNoFollow opens the final component without following a symbolic link and bounds the read.
func (osFS) ReadFileNoFollow(name string, max int64) ([]byte, error) {
	fd, err := unix.Open(name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ELOOP) {
		return nil, platform.ErrFileSymlink
	}
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > max {
		return nil, platform.ErrFileTooLarge
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a bounded regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, platform.ErrFileTooLarge
	}
	return data, nil
}
