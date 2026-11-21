//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package real

import "os"

func openSnapshotFile(root *os.Root, name string) (*os.File, error) { return root.Open(name) }
