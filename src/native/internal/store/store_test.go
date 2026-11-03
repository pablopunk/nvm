package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestOpenCreatesDirectoryAndRejectsEmptyPath(t *testing.T) {
	if _, err := Open(fake.NewFS(), fake.NewClock(), ""); err == nil {
		t.Fatal("Open with an empty directory succeeded")
	}
	files := fake.NewFS()
	if _, err := Open(files, fake.NewClock(), testDir); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := files.Stat(testDir); err != nil {
		t.Fatalf("Open did not create the directory: %v", err)
	}
}

func TestMissingFileYieldsDefaultsWithoutWriting(t *testing.T) {
	s, files, _ := newFixture(t)
	got, err := settingsDocument(s).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Title != "default" || !reflect.DeepEqual(got.Items, []string{"alpha"}) {
		t.Fatalf("Load = %+v, want the defaults", got)
	}
	if _, err := files.Stat(settingsPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load wrote a file: %v", err)
	}
}

func TestFlushWritesIndentedEnvelopeWithSchemaVersion(t *testing.T) {
	s, files, _ := newFixture(t)
	doc := settingsDocument(s)
	if err := doc.Update(func(v *settingsDoc) { v.Title = "hello"; v.Count = 3 }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := doc.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	raw := mustRead(t, files, settingsPath)
	if !strings.Contains(string(raw), "\n  \"schemaVersion\": 1,") || !bytes.HasSuffix(raw, []byte("\n")) {
		t.Fatalf("file is not an indented envelope:\n%s", raw)
	}
	version, data := envelopeOf(t, raw)
	var got settingsDoc
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("data: %v", err)
	}
	if version != 1 || got.Title != "hello" || got.Count != 3 {
		t.Fatalf("version %d, data %+v", version, got)
	}
}

func TestWrittenFilesArePrivateByDefault(t *testing.T) {
	s, files, _ := newFixture(t)
	doc := settingsDocument(s)
	if err := doc.Update(func(v *settingsDoc) { v.Count = 1 }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := doc.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	info, err := files.Stat(settingsPath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestWithFileModeChangesPermissionBits(t *testing.T) {
	s, files, _ := newFixture(t, WithFileMode(0o644))
	doc := settingsDocument(s)
	if err := doc.Update(func(v *settingsDoc) { v.Count = 1 }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := doc.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	info, _ := files.Stat(settingsPath)
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o, want 0644", info.Mode().Perm())
	}
}

func TestLoadReturnsIndependentCopies(t *testing.T) {
	s, _, _ := newFixture(t)
	doc := settingsDocument(s)
	if err := doc.Update(func(v *settingsDoc) { v.Items = append(v.Items, "beta") }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	copied, err := doc.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	copied.Items[0] = "mutated"
	copied.Title = "changed"
	again, _ := doc.Load()
	if again.Items[0] != "alpha" || again.Title != "default" {
		t.Fatalf("mutating a loaded copy changed the document: %+v", again)
	}
}

func TestSaveSkipsWriteWhenContentIsUnchanged(t *testing.T) {
	saves := 0
	s, _, _ := newFixture(t, WithOnSaved(func(string) { saves++ }))
	doc := settingsDocument(s)
	if err := doc.Update(func(v *settingsDoc) { v.Count = 1 }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	mustSave(t, doc)
	mustSave(t, doc)
	if err := doc.Update(func(v *settingsDoc) { v.Count = 1 }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := doc.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if saves != 1 {
		t.Fatalf("writes = %d, want 1 for one real change", saves)
	}
}

func TestDocPanicsOnBadConfiguration(t *testing.T) {
	s, _, _ := newFixture(t)
	settingsDocument(s)
	cases := map[string]func(){
		"duplicate name":    func() { settingsDocument(s) },
		"path separator":    func() { Doc(s, "../x.json", 1, freshSettingsDoc) },
		"hidden name":       func() { Doc(s, ".x.json", 1, freshSettingsDoc) },
		"zero version":      func() { Doc(s, "a.json", 0, freshSettingsDoc) },
		"nil defaults":      func() { Doc[settingsDoc](s, "b.json", 1, nil) },
		"missing migration": func() { Doc(s, "c.json", 2, freshSettingsDoc) },
		"misnumbered step":  func() { Doc(s, "d.json", 2, freshSettingsDoc, Migration{From: 2, Up: passThrough}) },
		"step without Up":   func() { Doc(s, "e.json", 2, freshSettingsDoc, Migration{From: 1}) },
		"nested secret":     func() { Doc(s, "f.json", 1, func() nestedSecret { return nestedSecret{} }) },
		"unexported secret": func() { Doc(s, "g.json", 1, func() unexportedSecret { return unexportedSecret{key: canary} }) },
		"secret in a slice": func() { Doc(s, "h.json", 1, func() sliceSecret { return sliceSecret{} }) },
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected a panic for bad configuration")
				}
			}()
			build()
		})
	}
}

// nestedSecret hides a secret one level down, which the store refuses to support.
type nestedSecret struct {
	Inner struct {
		Key string `store:"secret"`
	}
}

// unexportedSecret tags an unexported field, which could not be omitted reliably.
type unexportedSecret struct {
	key string `store:"secret"`
}

// sliceSecret hides a secret inside a slice element.
type sliceSecret struct {
	Entries []struct {
		Key string `store:"secret"`
	}
}

func TestUpdateAfterCloseReturnsErrClosed(t *testing.T) {
	s, err := Open(fake.NewFS(), fake.NewClock(), testDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	doc := settingsDocument(s)
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := doc.Update(func(v *settingsDoc) { v.Count = 2 }); !errors.Is(err, ErrClosed) {
		t.Fatalf("Update after Close: %v, want ErrClosed", err)
	}
	if err := doc.Save(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Save after Close: %v, want ErrClosed", err)
	}
	if _, open := <-doc.Subscribe(); open {
		t.Fatal("Subscribe after Close returned an open channel")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, open := <-s.Notices(); open {
		t.Fatal("Notices channel is open after Close")
	}
}

func TestSubscribeCoalescesChangeSignals(t *testing.T) {
	s, _, _ := newFixture(t)
	doc := settingsDocument(s)
	events := doc.Subscribe()
	if err := doc.Update(func(v *settingsDoc) { v.Count = 1 }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	select {
	case <-events:
	default:
		t.Fatal("no change event after Update")
	}
	for range 2 {
		if err := doc.Update(func(v *settingsDoc) { v.Count++ }); err != nil {
			t.Fatalf("Update: %v", err)
		}
	}
	select {
	case <-events:
	default:
		t.Fatal("no change event after two Updates")
	}
	select {
	case <-events:
		t.Fatal("slow reader received two pending signals")
	default:
	}
}

// mustSave saves the document and fails the test on error.
func mustSave(t *testing.T, doc *Collection[settingsDoc]) {
	t.Helper()
	if err := doc.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
}
