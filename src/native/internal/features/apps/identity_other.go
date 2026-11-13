//go:build !unix

package apps

import "io/fs"

// statIdentity reports no identity on systems without POSIX stat records; the uninstall guards refuse such items.
func statIdentity(fs.FileInfo) (dev, ino uint64, uid int, ok bool) {
	return 0, 0, 0, false
}
