package app

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/search"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"
	appfeat "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// The J29 data set of PLAN.md section 3.1: 500 applications, 50k files and 5k clipboard items.
const (
	benchAppCount  = 500
	benchFileCount = 50_000
	benchClipCount = 5_000
)

// benchQueries are the keystroke queries the search benchmarks type: a two-letter prefix that matches
// applications, files and clipboard text; a longer prefix; and a phrase that matches almost nothing.
var benchQueries = []string{"cl", "clip", "show the time"}

// benchOutputs keeps results alive so the compiler cannot discard the work a benchmark measures.
var (
	benchItems []search.Item
	benchPaths []string
)

// benchDataSet is generated once per test binary and shared read-only by the benchmarks.
type benchDataSet struct {
	apps  []appfeat.App
	files []string
	clips []ClipEntry
}

var benchData = sync.OnceValue(func() benchDataSet {
	var apps []appfeat.App
	for _, fixture := range fixtures.Apps(benchAppCount) {
		apps = append(apps, appfeat.App{Name: fixture.App.Name, ID: fixture.App.ID, Path: fixture.App.Path})
	}
	return benchDataSet{
		apps:  apps,
		files: fixtures.Files(benchFileCount),
		clips: benchClipEntries(fixtures.ClipboardEntries(benchClipCount)),
	}
})

// benchClipEntries converts generated copies into the history the app keeps. Concealed copies are
// dropped, as the app never records them.
func benchClipEntries(entries []fixtures.ClipboardEntry) []ClipEntry {
	out := make([]ClipEntry, 0, len(entries))
	for i, entry := range entries {
		if entry.Concealed {
			continue
		}
		if entry.PNG != nil {
			out = append(out, ClipEntry{ID: fmt.Sprintf("img-%d", i), Kind: "image", Image: fmt.Sprintf("img-%d.png", i), W: 16, H: 16, At: entry.At})
			continue
		}
		sum := sha1.Sum([]byte(entry.Text))
		out = append(out, ClipEntry{ID: hex.EncodeToString(sum[:8]), Kind: "text", Text: entry.Text, At: entry.At})
	}
	return out
}

// newBenchPalette returns a palette whose stores hold the J29 data set and whose root view is the
// one the app opens. Settings, image and chat stores read and write under a temporary HOME, so the
// benchmarks never touch the user's configuration.
func newBenchPalette(b *testing.B) *App {
	b.Helper()
	b.Setenv("HOME", b.TempDir())
	data := benchData()
	p := &App{
		S:      &Settings{Hotkey: "Alt+Shift+Space", Engine: "google", ClipLimit: benchClipCount, Disabled: map[string]bool{}},
		appsvc: newFixtureAppServices(data.apps),
		clip:   &ClipStore{Entries: data.clips, thumbs: map[string]*ui.Bitmap{}},
		snips:  &SnippetStore{},
		chats:  &ChatStore{},
		files:  &FileIndex{paths: data.files, done: true},
	}
	p.newPalette(searchClock, true)
	p.SetStack(p.rootView())
	return p
}
