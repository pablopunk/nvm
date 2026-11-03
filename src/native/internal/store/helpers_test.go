package store

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// testDir is the store directory inside the in-memory file system.
const testDir = "/cfg/nvm"

// settingsPath is the file the settings document is written to.
const settingsPath = testDir + "/settings.json"

// settingsDoc is the document most tests use. Token is a secret: it must never reach disk.
type settingsDoc struct {
	Title string   `json:"title"`
	Count int      `json:"count"`
	Items []string `json:"items,omitempty"`
	Token string   `json:"token,omitempty" store:"secret"`
}

// freshSettingsDoc is the defaults function of the settings document.
func freshSettingsDoc() settingsDoc {
	return settingsDoc{Title: "default", Items: []string{"alpha"}}
}

// newFixture opens a store on a fresh fake file system with a manual clock. Debounce defaults to an hour so
// that tests write only when they flush, close or advance the clock on purpose.
func newFixture(t *testing.T, opts ...Option) (*Store, *fake.FS, *fake.Clock) {
	t.Helper()
	files := fake.NewFS()
	clock := fake.NewClock()
	all := append([]Option{WithDebounce(time.Hour)}, opts...)
	s, err := Open(files, clock, testDir, all...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, files, clock
}

// settingsDocument registers the settings document in s.
func settingsDocument(s *Store) *Collection[settingsDoc] {
	return Doc(s, "settings.json", 1, freshSettingsDoc)
}

// mustRead returns a file's bytes or fails the test.
func mustRead(t *testing.T, files *fake.FS, path string) []byte {
	t.Helper()
	data, err := files.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	return data
}

// envelopeOf decodes a stored file into its schema version and raw data.
func envelopeOf(t *testing.T, data []byte) (int, json.RawMessage) {
	t.Helper()
	var env typedEnvelope[json.RawMessage]
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("stored file is not an envelope: %v\n%s", err, data)
	}
	return env.SchemaVersion, env.Data
}

// drainNotices returns the notices waiting on ch without blocking.
func drainNotices(ch <-chan Notice) []Notice {
	var out []Notice
	for {
		select {
		case notice, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, notice)
		default:
			return out
		}
	}
}

// findNotice returns the first notice of kind, if any.
func findNotice(notices []Notice, kind Kind) (Notice, bool) {
	for _, notice := range notices {
		if notice.Kind == kind {
			return notice, true
		}
	}
	return Notice{}, false
}

// quarantinedFiles lists the corrupt copies left in the store directory.
func quarantinedFiles(files *fake.FS) []string {
	var out []string
	for _, path := range files.Paths() {
		if strings.Contains(path, ".corrupt-") {
			out = append(out, path)
		}
	}
	return out
}

// readTestdata loads a fixture from testdata/ next to this package.
func readTestdata(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}
