package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
)

// legacyStatePath writes a 1.x state.json that records acme-theme.ts as on or off, and returns its path.
func legacyStatePath(t *testing.T, on bool) string {
	t.Helper()
	state := `{"extensionManager":{"schemaVersion":1,"files":{"acme-theme.ts":{"enabled":` + map[bool]string{true: "true", false: "false"}[on] + `}}}}`
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// placeFolder copies the fixture folder into the extensions root, as a converted folder appears on disk.
func (h *harness) placeFolder(t *testing.T, name string) {
	t.Helper()
	dir := filepath.Join(h.root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for file, contents := range fixtureContents(t, name) {
		if err := os.WriteFile(filepath.Join(dir, file), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestConvertedFolderInheritsAnEnabled1xFile: a folder converted from a 1.x file that was on starts on its first scan,
// with no review, as the 1.x file did.
func TestConvertedFolderInheritsAnEnabled1xFile(t *testing.T) {
	statePath := legacyStatePath(t, true)
	h := newHarness(t, func(o *Options) { o.LegacyStatePath = statePath })
	h.placeFolder(t, "acme.theme")
	if err := h.m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if listing := h.listing("acme.theme"); !listing.Enabled {
		t.Fatal("the converted folder is off; the 1.x file it came from was on")
	}
	h.waitRunning("acme.theme")
}

// TestInheritanceRunsOnceAndKeepsALaterChoice: after the user turns the converted folder off here, a later scan does not
// turn it back on from the 1.x file.
func TestInheritanceRunsOnceAndKeepsALaterChoice(t *testing.T) {
	statePath := legacyStatePath(t, true)
	h := newHarness(t, func(o *Options) { o.LegacyStatePath = statePath })
	h.placeFolder(t, "acme.theme")
	if err := h.m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Disable("acme.theme"); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if h.listing("acme.theme").Enabled {
		t.Fatal("a later scan turned a disabled converted folder back on from 1.x")
	}
}

// TestA1xFileThatWasOffLeavesTheFolderOffButRecorded: a 1.x file that was off is inherited as off, so the folder is
// listed as disabled and waits for the user to review it.
func TestA1xFileThatWasOffLeavesTheFolderOffButRecorded(t *testing.T) {
	statePath := legacyStatePath(t, false)
	h := newHarness(t, func(o *Options) { o.LegacyStatePath = statePath })
	h.placeFolder(t, "acme.theme")
	if err := h.m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if h.listing("acme.theme").Enabled {
		t.Fatal("the converted folder is on; the 1.x file it came from was off")
	}
	if done, err := h.enabled.Inherited("acme.theme"); err != nil || !done {
		t.Fatalf("Inherited = %v, %v; want the inheritance recorded", done, err)
	}
}

// TestNo1xStateMeansNothingIsInheritedYet: without a 1.x state file the folder is off and nothing is recorded, so a
// later scan that finds the file still inherits its state.
func TestNo1xStateMeansNothingIsInheritedYet(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	h := newHarness(t, func(o *Options) { o.LegacyStatePath = statePath })
	h.placeFolder(t, "acme.theme")
	if err := h.m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if h.listing("acme.theme").Enabled {
		t.Fatal("the converted folder is on with no 1.x state to inherit from")
	}
	if done, _ := h.enabled.Inherited("acme.theme"); done {
		t.Fatal("the inheritance was recorded with no 1.x state read")
	}
	if err := os.WriteFile(statePath, []byte(`{"extensionManager":{"files":{"acme-theme.ts":{"enabled":true}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if !h.listing("acme.theme").Enabled {
		t.Fatal("the later scan did not inherit the 1.x state once it existed")
	}
}

// TestFoldersWithoutConvertedFromAreNotInherited: a folder that names no 1.x file keeps the default, off.
func TestFoldersWithoutConvertedFromAreNotInherited(t *testing.T) {
	statePath := legacyStatePath(t, true)
	h := newHarness(t, func(o *Options) { o.LegacyStatePath = statePath })
	h.placeFolder(t, "acme.theme")
	manifest := filepath.Join(h.root, "acme.theme", bundle.ManifestFile)
	if err := os.WriteFile(manifest, []byte(`{"apiVersion":"1.0","id":"acme.theme","title":"Acme Theme","capabilities":["system"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := h.m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if h.listing("acme.theme").Enabled {
		t.Fatal("a folder with no convertedFrom inherited a 1.x state")
	}
}
