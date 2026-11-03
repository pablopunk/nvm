package spike

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

// fakeFS is an in-memory Resolver: directories, files and symlinks under absolute paths. Canonical walks the path one
// component at a time, following links, and fails for a component that does not exist, as EvalSymlinks does.
type fakeFS struct {
	dirs  map[string]bool
	files map[string]bool
	links map[string]string
}

var errFakeLoop = errors.New("symlink loop")

func newFakeFS() *fakeFS {
	return &fakeFS{dirs: map[string]bool{"/": true}, files: map[string]bool{}, links: map[string]string{}}
}

func (f *fakeFS) addDir(path string) *fakeFS {
	f.dirs[filepath.Clean(path)] = true
	return f
}

func (f *fakeFS) addFile(path string) *fakeFS {
	f.files[filepath.Clean(path)] = true
	return f
}

func (f *fakeFS) addLink(path, target string) *fakeFS {
	f.links[filepath.Clean(path)] = filepath.Clean(target)
	return f
}

func (f *fakeFS) exists(path string) bool {
	return f.dirs[path] || f.files[path]
}

// Canonical implements Resolver.
func (f *fakeFS) Canonical(target string) (string, error) {
	pending := splitComponents(filepath.Clean(target))
	current := "/"
	for hops := 0; len(pending) > 0; hops++ {
		if hops > 256 {
			return "", errFakeLoop
		}
		next := filepath.Join(current, pending[0])
		pending = pending[1:]
		if link, ok := f.links[next]; ok {
			pending = append(splitComponents(link), pending...)
			current = "/"
			continue
		}
		if !f.exists(next) {
			return "", fs.ErrNotExist
		}
		current = next
	}
	return current, nil
}

func splitComponents(path string) []string {
	trimmed := strings.TrimPrefix(filepath.Clean(path), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

// guardFixture is the directory layout of ai-credential-path-guard.test.ts, rebuilt in memory.
type guardFixture struct {
	fs        *fakeFS
	home      string
	documents string
	elsewhere string
}

func newGuardFixture() guardFixture {
	const home = "/fx/home"
	fake := newFakeFS().
		addDir("/fx").
		addDir(home).
		addDir(home + "/.ssh").
		addFile(home + "/.ssh/id_ed25519").
		addDir(home + "/Documents").
		addFile(home + "/Documents/notes.md").
		addDir("/fx/elsewhere")
	return guardFixture{fs: fake, home: home, documents: home + "/Documents", elsewhere: "/fx/elsewhere"}
}
