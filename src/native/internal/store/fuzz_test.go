package store

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// FuzzLoad feeds arbitrary bytes to a document's first load. The properties: loading never panics, and the
// original bytes survive, either as the file itself (parsed or read-only) or as a quarantined or
// pre-migration copy. The document has a migration so that version-1 inputs exercise that path too.
func FuzzLoad(f *testing.F) {
	for _, name := range []string{"v1_settings.json", "truncated_settings.json", "empty_settings.json", "wrong_envelope.json", "newer_settings.json"} {
		f.Add(readTestdata(f, name))
	}
	f.Add([]byte(`{"schemaVersion":1,"data":{"title":"x","count":1}}`))
	f.Add([]byte(`{"schemaVersion":2,"data":null}`))
	f.Add([]byte("\x00\xff{"))
	f.Fuzz(func(t *testing.T, input []byte) {
		files := fake.NewFS()
		s, err := Open(files, fake.NewClock(), testDir, WithDebounce(time.Hour))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer func() { _ = s.Close() }()
		files.Seed(settingsPath, input)
		doc := Doc(s, "settings.json", 2, freshSettingsDoc, Migration{From: 1, Up: passThrough})
		_, _ = doc.Load()
		if !originalSurvives(files, input) {
			t.Fatalf("the original bytes were lost; input %q", input)
		}
	})
}

// passThrough is a migration step that changes nothing, for tests that only need a chain to exist.
func passThrough(data json.RawMessage) (json.RawMessage, error) {
	return data, nil
}

// originalSurvives reports whether some file in the store holds exactly the input bytes.
func originalSurvives(files *fake.FS, input []byte) bool {
	for _, path := range files.Paths() {
		if isDirectory(files, path) {
			continue
		}
		data, err := files.ReadFile(path)
		if err == nil && bytes.Equal(data, input) {
			return true
		}
	}
	return false
}
