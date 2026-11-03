package files

import (
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// NewIndexOfPaths returns an index that already serves paths, as if a scan had committed them. Fixtures and benchmarks
// use it: the index never scans, watches or saves, and its status reports ready from the persisted copy. Paths are
// native paths; each file is recorded with no size or dates.
func NewIndexOfPaths(fsys platform.FS, clock platform.Clock, home string, paths []string) (*Index, error) {
	ix, err := New(Config{Home: home, Roots: []string{home}, FS: fsys, Clock: clock})
	if err != nil {
		return nil, err
	}
	b := newBuilder(len(paths))
	for _, p := range paths {
		b.addRecord(record{dir: filepath.Dir(p), name: filepath.Base(p)})
	}
	ix.publish(b.build(1, ix.configKey), true)
	return ix, nil
}
