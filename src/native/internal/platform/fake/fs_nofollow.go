package fake

import (
	"bytes"
	"github.com/pablopunk/nvm/native/internal/platform"
	"io/fs"
)

// ReadFileNoFollow reads a bounded regular file without resolving its final link.
func (f *FS) ReadFileNoFollow(name string, max int64) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.nodes[cleanKey(name)]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if n.link != "" {
		return nil, platform.ErrFileSymlink
	}
	if int64(len(n.data)) > max {
		return nil, platform.ErrFileTooLarge
	}
	if n.dir || !n.mode.IsRegular() {
		return nil, fs.ErrInvalid
	}
	return bytes.Clone(n.data), nil
}
