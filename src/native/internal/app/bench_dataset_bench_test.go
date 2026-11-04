package app

import (
	"fmt"
	"sync"
	"testing"

	appfeat "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/search"
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
	clips []clipboard.Entry
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

// benchClipEntries converts generated copies into the entries the history keeps, newest first. Concealed copies
// are dropped, as the app never records them.
func benchClipEntries(entries []fixtures.ClipboardEntry) []clipboard.Entry {
	out := make([]clipboard.Entry, 0, len(entries))
	for i, entry := range entries {
		if entry.Concealed {
			continue
		}
		if entry.PNG != nil {
			out = append(out, clipboard.Entry{ID: fmt.Sprintf("image:%d", i), Kind: clipboard.KindImage, ImageRef: fmt.Sprintf("%040x.png", i), Width: 16, Height: 16, CreatedAt: entry.At})
			continue
		}
		if id := clipboard.TextID(entry.Text); id != "" {
			out = append(out, clipboard.Entry{ID: id, Kind: clipboard.KindText, Text: entry.Text, CreatedAt: entry.At})
		}
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
		clip:   benchClipboard(b, data.clips),

		files: newTestFiles(b, b.TempDir(), data.files),
	}
	p.newPalette(searchClock, true)
	p.SetStack(p.rootView())
	return p
}
