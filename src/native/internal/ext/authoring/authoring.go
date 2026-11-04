// Package authoring changes the folder of an installed extension: Duplicate writes a copy under a new id, and Rename
// changes the title its manifest declares. Both work on extension.json and main.go only, so no source is rewritten.
package authoring

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// maxAttempts bounds the ids Duplicate tries when a folder with the derived name already exists.
const maxAttempts = 100

// Duplicate copies the folder dir, which lives under root, into a new folder with a new id and the title "Copy of <title>",
// as 1.x names a duplicate. The copy's id is the original id with a suffix derived from the time. A folder that already
// has the derived name is skipped, and the next suffix is tried. It returns the new id.
func Duplicate(files platform.FS, root, dir string, now time.Time) (string, error) {
	manifest, source, err := readFolder(files, dir)
	if err != nil {
		return "", err
	}
	id, _ := manifest["id"].(string)
	title, _ := manifest["title"].(string)
	for attempt := range maxAttempts {
		newID := id + "-copy-" + suffix(id, now, attempt)
		newDir := filepath.Join(root, newID)
		if taken(files, newDir) {
			continue
		}
		copied := copyOf(manifest, newID, "Copy of "+title)
		if err := writeFolder(files, newDir, copied, source); err != nil {
			return "", err
		}
		return newID, nil
	}
	return "", fmt.Errorf("no free folder name for a copy of %s", id)
}

// Rename sets the title the manifest of the folder dir declares. The id and the source stay as they are.
func Rename(files platform.FS, dir, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("name is required")
	}
	manifest, _, err := readFolder(files, dir)
	if err != nil {
		return err
	}
	manifest["title"] = title
	data, err := encodeManifest(manifest)
	if err != nil {
		return err
	}
	return replaceFile(files, filepath.Join(dir, bundle.ManifestFile), data)
}

func readFolder(files platform.FS, dir string) (map[string]any, []byte, error) {
	data, err := files.ReadFile(filepath.Join(dir, bundle.ManifestFile))
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", bundle.ManifestFile, err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, nil, fmt.Errorf("%s is not valid JSON: %w", bundle.ManifestFile, err)
	}
	source, err := files.ReadFile(filepath.Join(dir, bundle.SourceFile))
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", bundle.SourceFile, err)
	}
	return manifest, source, nil
}

func copyOf(manifest map[string]any, id, title string) map[string]any {
	out := make(map[string]any, len(manifest))
	for key, value := range manifest {
		out[key] = value
	}
	out["id"] = id
	out["title"] = title
	return out
}

func writeFolder(files platform.FS, dir string, manifest map[string]any, source []byte) error {
	if err := files.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := encodeManifest(manifest)
	if err != nil {
		return err
	}
	if err := files.CreateExclusive(filepath.Join(dir, bundle.ManifestFile), data, 0o644); err != nil {
		return err
	}
	return files.WriteFile(filepath.Join(dir, bundle.SourceFile), source, 0o644)
}

// replaceFile writes data over name through a temporary file, so a crash never leaves a half-written manifest.
func replaceFile(files platform.FS, name string, data []byte) error {
	temp := name + ".tmp"
	if err := files.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	return files.Rename(temp, name)
}

func encodeManifest(manifest map[string]any) ([]byte, error) {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// suffix is the first eight hex digits of a hash of the id, the time and the attempt, as 1.x derives its copy ids.
func suffix(id string, now time.Time, attempt int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", id, now.UnixNano(), attempt)))
	return hex.EncodeToString(sum[:])[:8]
}

func taken(files platform.FS, dir string) bool {
	_, err := files.Stat(dir)
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}
