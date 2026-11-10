package install

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/platform"
)

const maxSnapshotBytes int64 = 2 << 20
const maxSnapshotFiles = 64

// SourceSnapshot reads every file that a folder replacement would remove. It refuses folders containing
// other entries rather than dropping them. The caller must serialize cooperating installs and revalidate
// immediately before publication; this function does not lock out external filesystem writers.
func SourceSnapshot(files platform.FS, root, id string) (map[string][]byte, error) {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\`) {
		return nil, fmt.Errorf("invalid extension folder name %q", id)
	}
	dir := filepath.Join(root, id)
	info, err := files.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("extension folder is not a directory")
	}
	entries, err := files.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	if len(entries) > maxSnapshotFiles {
		return nil, fmt.Errorf("extension has too many files")
	}
	result := make(map[string][]byte, len(entries))
	remaining := maxSnapshotBytes
	for _, entry := range entries {
		name := entry.Name()
		if name != bundle.ManifestFile && !strings.HasSuffix(name, ".go") {
			return nil, fmt.Errorf("unsupported extension entry %q", name)
		}
		path := filepath.Join(dir, name)
		info, err := files.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("extension entry %q is not a regular file", name)
		}
		data, size, err := files.ReadFilePrefix(path, remaining+1)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", name, err)
		}
		if size > remaining || int64(len(data)) != size {
			return nil, fmt.Errorf("extension source exceeds snapshot limit or changed while reading")
		}
		remaining -= size
		result[name] = data
	}
	if _, ok := result[bundle.ManifestFile]; !ok {
		return nil, fmt.Errorf("extension manifest is missing")
	}
	if _, ok := result[bundle.SourceFile]; !ok {
		return nil, fmt.Errorf("extension main source is missing")
	}
	return result, nil
}
