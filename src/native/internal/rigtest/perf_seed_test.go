//go:build rig

package rigtest

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// The J29 scale row of PLAN.md section 3.1 for the clipboard and the files: 5,000 clipboard items and
// 50,000 files under the isolated HOME. Applications are not seeded: the rig desktop's own list is
// what the palette shows, so application counts in the rig are not the 500 of the Go benchmarks.
const (
	j29ClipboardItems = 5000
	j29Files          = 50000
)

// seededClip mirrors the fields of the app's clipboard history file that a text entry needs.
type seededClip struct {
	ID   string
	Kind string
	Text string
	At   time.Time
}

// seedJ29 writes the clipboard history and the file tree of the J29 data set into home. The app
// reads the history at start and indexes the files two seconds after ready.
func seedJ29(t *testing.T, home string) {
	t.Helper()
	data := rigDataDir(home)
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	seedClipboardHistory(t, data)
	// The app trims history to its limit when a new copy arrives; the limit must hold the seeded items.
	if err := os.WriteFile(filepath.Join(data, "settings.json"), []byte(`{"ClipLimit": 5000}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	seedFileTree(t, home)
}

// seedClipboardHistory writes text entries only; images need the app's image directory, and the
// concealed copies the app never records are dropped, as in the app.
func seedClipboardHistory(t *testing.T, config string) {
	t.Helper()
	var history struct {
		Entries []seededClip
	}
	for _, entry := range fixtures.ClipboardEntries(j29ClipboardItems) {
		if entry.Concealed || entry.PNG != nil {
			continue
		}
		sum := sha1.Sum([]byte(entry.Text))
		history.Entries = append(history.Entries, seededClip{
			ID:   hex.EncodeToString(sum[:8]),
			Kind: "text",
			Text: entry.Text,
			At:   entry.At,
		})
	}
	data, err := json.Marshal(history)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "clipboard.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// seedFileTree creates each generated path under home, with the generator's home directory replaced
// by home. Files are empty: the index reads names, not contents.
func seedFileTree(t *testing.T, home string) {
	t.Helper()
	made := map[string]bool{}
	for _, path := range fixtures.Files(j29Files) {
		target := filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(path, "/home/user/")))
		dir := filepath.Dir(target)
		if !made[dir] {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			made[dir] = true
		}
		if err := os.WriteFile(target, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
