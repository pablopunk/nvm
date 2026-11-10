package install

import (
	"errors"
	"fmt"
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
	capability, ok := files.(platform.SourceDirectories)
	if !ok {
		return nil, platform.ErrUnsupported
	}
	dir, err := capability.OpenSourceDirectory(root, id)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.Entries(maxSnapshotFiles)
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
		if entry.IsDir() {
			return nil, fmt.Errorf("extension entry %q is not a regular file", name)
		}
		data, err := dir.ReadFile(name, remaining)
		if err != nil {
			if errors.Is(err, platform.ErrFileTooLarge) {
				return nil, fmt.Errorf("extension source exceeds snapshot limit")
			}
			return nil, fmt.Errorf("read %q: %w", name, err)
		}
		if int64(len(data)) > remaining {
			return nil, fmt.Errorf("extension source exceeds snapshot limit")
		}
		remaining -= int64(len(data))
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
