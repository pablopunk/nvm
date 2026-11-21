//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package real

import (
	"errors"
	"os"

	"github.com/pablopunk/nvm/native/internal/platform"
	"golang.org/x/sys/unix"
)

// OpenReadNoFollow authorizes a regular source without following its final symbolic link.
func (osFS) OpenReadNoFollow(name string) (platform.ReadSource, error) {
	fd, err := unix.Open(name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ELOOP) {
		return nil, platform.ErrFileSymlink
	}
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	accepted := false
	defer func() {
		if !accepted {
			_ = f.Close()
		}
	}()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a bounded regular file")
	}
	accepted = true
	return &readSource{File: f, identity: sourceFileIdentity(info)}, nil
}
