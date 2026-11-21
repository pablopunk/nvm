package folders

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAdoptRenamedMovesTheEarlierFolderOnce(t *testing.T) {
	root := t.TempDir()
	previous, current := filepath.Join(root, "Nevermind Next"), filepath.Join(root, "Nevermind")
	writeFile(t, filepath.Join(previous, "settings.json"), "earlier")
	moved, err := AdoptRenamed(previous, current)
	if err != nil || !moved {
		t.Fatalf("AdoptRenamed = %v, %v; want moved", moved, err)
	}
	if got := readFile(t, filepath.Join(current, "settings.json")); got != "earlier" {
		t.Fatalf("settings.json = %q; want the earlier folder's file", got)
	}
	if _, err := os.Stat(previous); !os.IsNotExist(err) {
		t.Fatalf("earlier folder still there: %v", err)
	}
	if moved, err := AdoptRenamed(previous, current); err != nil || moved {
		t.Fatalf("second AdoptRenamed = %v, %v; want nothing to do", moved, err)
	}
}

func TestAdoptRenamedKeepsAnExistingCurrentFolder(t *testing.T) {
	root := t.TempDir()
	previous, current := filepath.Join(root, "nevermind-next"), filepath.Join(root, "nevermind")
	writeFile(t, filepath.Join(previous, "settings.json"), "earlier")
	writeFile(t, filepath.Join(current, "settings.json"), "current")
	if moved, err := AdoptRenamed(previous, current); err != nil || moved {
		t.Fatalf("AdoptRenamed = %v, %v; want nothing moved", moved, err)
	}
	if got := readFile(t, filepath.Join(current, "settings.json")); got != "current" {
		t.Fatalf("settings.json = %q; want the current folder untouched", got)
	}
	if got := readFile(t, filepath.Join(previous, "settings.json")); got != "earlier" {
		t.Fatalf("earlier settings.json = %q; want it left alone", got)
	}
}

func TestAdoptRenamedWithoutAnEarlierFolder(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "nevermind")
	if moved, err := AdoptRenamed(filepath.Join(root, "nevermind-next"), current); err != nil || moved {
		t.Fatalf("AdoptRenamed = %v, %v; want nothing to do", moved, err)
	}
	if _, err := os.Stat(current); !os.IsNotExist(err) {
		t.Fatalf("current folder created: %v", err)
	}
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
