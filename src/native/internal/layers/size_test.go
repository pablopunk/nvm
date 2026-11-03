package layers

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

const (
	moduleRoot         = "../.."
	sizeBaselinePath   = "size-baseline.json"
	maxUnbaselineLines = 400
)

// readSizeBaseline loads the grandfathered line counts, keyed by slash path relative to src/native.
// Only files above maxUnbaselineLines belong here; the counts may only go down.
func readSizeBaseline(t *testing.T) map[string]int {
	t.Helper()
	data, err := os.ReadFile(sizeBaselinePath)
	if err != nil {
		t.Fatalf("read %s: %v", sizeBaselinePath, err)
	}
	var baseline map[string]int
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatalf("parse %s: %v", sizeBaselinePath, err)
	}
	return baseline
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, []byte("\n"))
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}

func TestGoFilesStayWithinSizeRatchet(t *testing.T) {
	baseline := readSizeBaseline(t)
	seen := map[string]bool{}

	err := filepath.WalkDir(moduleRoot, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(file) != ".go" {
			return nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(moduleRoot, file)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(relative)
		seen[key] = true
		checkFileAgainstSizeRatchet(t, key, countLines(data), baseline)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", moduleRoot, err)
	}

	for key := range baseline {
		if !seen[key] {
			t.Errorf("%s is in %s but no longer exists: remove its entry", key, sizeBaselinePath)
		}
	}
}

func checkFileAgainstSizeRatchet(t *testing.T, key string, lines int, baseline map[string]int) {
	t.Helper()
	allowed, grandfathered := baseline[key]
	switch {
	case !grandfathered && lines > maxUnbaselineLines:
		t.Errorf("%s has %d lines (limit %d) and no baseline entry: split the file instead", key, lines, maxUnbaselineLines)
	case grandfathered && lines > allowed:
		t.Errorf("%s grew to %d lines (baseline %d): move code out instead of growing it", key, lines, allowed)
	case grandfathered && lines <= maxUnbaselineLines:
		t.Errorf("%s is %d lines, within the %d-line limit: remove its entry from %s", key, lines, maxUnbaselineLines, sizeBaselinePath)
	case grandfathered && lines < allowed:
		t.Errorf("%s shrank to %d lines (baseline %d): lower its entry in %s to %d", key, lines, allowed, sizeBaselinePath, lines)
	}
}
