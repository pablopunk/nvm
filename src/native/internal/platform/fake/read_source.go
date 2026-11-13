package fake

import (
	"bytes"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/platform"
	"io/fs"
	"path"
)

const OpOpenReadNoFollow = "OpenReadNoFollow"

type readSource struct {
	*bytes.Reader
	info     fs.FileInfo
	identity string
}

func (s *readSource) Close() error               { return nil }
func (s *readSource) Stat() (fs.FileInfo, error) { return s.info, nil }
func (s *readSource) Identity() string           { return s.identity }

func (f *FS) OpenReadNoFollow(name string) (platform.ReadSource, error) {
	if err := f.faults.take(OpOpenReadNoFollow); err != nil {
		return nil, err
	}
	if err := f.faults.takePath(OpOpenReadNoFollow, name); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := cleanKey(name)
	n, ok := f.nodes[key]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if n.link != "" {
		return nil, platform.ErrFileSymlink
	}
	if n.dir {
		return nil, fs.ErrInvalid
	}
	if n.mode.Perm()&0o444 == 0 {
		return nil, fs.ErrPermission
	}
	snapshot := *n
	snapshot.data = bytes.Clone(n.data)
	return &readSource{Reader: bytes.NewReader(snapshot.data), info: fileInfo{name: path.Base(key), node: &snapshot}, identity: fmt.Sprintf("%p", n)}, nil
}
