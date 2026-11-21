package authoring

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

const root = "/home/owner/.config/nevermind/extensions"

func writeExtension(t *testing.T, files *fake.FS, id, title string) string {
	t.Helper()
	dir := root + "/" + id
	if err := files.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"apiVersion":"1.0","id":"` + id + `","title":"` + title + `","subtitle":"Keep me"}`
	if err := files.WriteFile(dir+"/"+bundle.ManifestFile, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile(dir+"/"+bundle.SourceFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func manifestOf(t *testing.T, files *fake.FS, dir string) map[string]any {
	t.Helper()
	data, err := files.ReadFile(dir + "/" + bundle.ManifestFile)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestDuplicateWritesACopyWithANewIDAndTitle checks the copy: its folder is named after its new id, its title is
// "Copy of" the original, its other manifest fields and its source are kept, and the original is untouched.
func TestDuplicateWritesACopyWithANewIDAndTitle(t *testing.T) {
	files := fake.NewFS()
	dir := writeExtension(t, files, "acme.notes", "Notes")
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	id, err := Duplicate(files, root, dir, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "acme.notes-copy-") {
		t.Fatalf("copy id = %q", id)
	}
	copyManifest := manifestOf(t, files, root+"/"+id)
	if copyManifest["id"] != id || copyManifest["title"] != "Copy of Notes" || copyManifest["subtitle"] != "Keep me" {
		t.Fatalf("copy manifest = %v", copyManifest)
	}
	if original := manifestOf(t, files, dir); original["id"] != "acme.notes" || original["title"] != "Notes" {
		t.Fatalf("original changed: %v", original)
	}
}

// TestDuplicateSkipsAFolderThatAlreadyHasTheDerivedName checks that a taken name is passed over for the next suffix.
func TestDuplicateSkipsAFolderThatAlreadyHasTheDerivedName(t *testing.T) {
	files := fake.NewFS()
	dir := writeExtension(t, files, "acme.notes", "Notes")
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	first := "acme.notes-copy-" + suffix("acme.notes", now, 0)
	if err := files.MkdirAll(root+"/"+first, 0o755); err != nil {
		t.Fatal(err)
	}
	id, err := Duplicate(files, root, dir, now)
	if err != nil {
		t.Fatal(err)
	}
	if id == first {
		t.Fatalf("the taken name %q was reused", id)
	}
}

// TestRenameChangesOnlyTheTitle checks that Rename writes the new title and keeps the id and the other fields.
func TestRenameChangesOnlyTheTitle(t *testing.T) {
	files := fake.NewFS()
	dir := writeExtension(t, files, "acme.notes", "Notes")
	if err := Rename(files, dir, "  Journal  "); err != nil {
		t.Fatal(err)
	}
	m := manifestOf(t, files, dir)
	if m["title"] != "Journal" || m["id"] != "acme.notes" || m["subtitle"] != "Keep me" {
		t.Fatalf("manifest after rename = %v", m)
	}
}

// TestRenameRefusesABlankTitle checks that a blank name changes nothing.
func TestRenameRefusesABlankTitle(t *testing.T) {
	files := fake.NewFS()
	dir := writeExtension(t, files, "acme.notes", "Notes")
	if err := Rename(files, dir, "   "); err == nil {
		t.Fatal("a blank name was accepted")
	}
	if manifestOf(t, files, dir)["title"] != "Notes" {
		t.Fatal("a refused rename changed the title")
	}
}

// TestRenameCommandSetsOnlyThatCommandTitle checks that a command rename adds its entry to commandTitles and leaves the
// extension's title and the other entries alone.
func TestRenameCommandSetsOnlyThatCommandTitle(t *testing.T) {
	files := fake.NewFS()
	dir := writeExtension(t, files, "acme.notes", "Notes")
	if err := RenameCommand(files, dir, "new", " Jot down "); err != nil {
		t.Fatal(err)
	}
	if err := RenameCommand(files, dir, "open", "Open notes"); err != nil {
		t.Fatal(err)
	}
	m := manifestOf(t, files, dir)
	titles, _ := m["commandTitles"].(map[string]any)
	if titles["new"] != "Jot down" || titles["open"] != "Open notes" || len(titles) != 2 {
		t.Fatalf("commandTitles = %v", titles)
	}
	if m["title"] != "Notes" {
		t.Fatalf("extension title changed to %v", m["title"])
	}
}

// TestRenameCommandRefusesABlankTitle checks that a blank command name changes nothing.
func TestRenameCommandRefusesABlankTitle(t *testing.T) {
	files := fake.NewFS()
	dir := writeExtension(t, files, "acme.notes", "Notes")
	if err := RenameCommand(files, dir, "new", "  "); err == nil {
		t.Fatal("a blank command name was accepted")
	}
}

func TestApplyChangesOnlyTheNamesItIsGiven(t *testing.T) {
	files := fake.NewFS()
	dir := writeExtension(t, files, "acme.names", "Old")
	if err := Apply(files, dir, "", "New subtitle", "run", "Go"); err != nil {
		t.Fatal(err)
	}
	m := manifestOf(t, files, dir)
	if m["title"] != "Old" || m["subtitle"] != "New subtitle" {
		t.Fatalf("manifest = %v", m)
	}
	if titles, _ := m["commandTitles"].(map[string]any); titles["run"] != "Go" {
		t.Fatalf("commandTitles = %v", m["commandTitles"])
	}
}
