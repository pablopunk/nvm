//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package real

import (
	"golang.org/x/sys/unix"
	"os"
)

// Nonblocking open keeps substitution of a FIFO from blocking a snapshot worker before Stat rejects it.
func openSnapshotFile(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|unix.O_NONBLOCK, 0)
}
