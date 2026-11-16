package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	appfeat "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/search/usage"
)

// loomNoiseFiles are the files the owner's index returned for "loom": each holds the letters of loom scattered through
// its name, and none of them is named for it.
var loomNoiseFiles = []string{
	"Documents/Adobe Lightroom.png",
	"Pictures/Asombroso_estrellas_de_la_nebulosa-NGC1234.png",
	"Documents/aurora_helix_homerow_mods.uf2",
	"Downloads/Apple_Developer_Program_License_Agreement_20240601.pdf",
	"Downloads/Apple_Developer_Program_License_Agreement_20250911.pdf",
	"Documents/CARTA ANULACION A VTO firmado seguro axa coche audi s3 .pdf",
}

// loomChosenFixture is the root with the Loom app, the six noise files in the file index, and Log in to and Log out of
// Nevermind chosen again and again for "lo" and "loo", the queries "loom" extends.
func loomChosenFixture(t *testing.T, extraFiles ...string) *App {
	t.Helper()
	p := rootTableFixtureWithApps(t, []appfeat.App{{Name: "Loom", ID: "com.loom.desktop", Path: "/usr/share/applications/loom.desktop"}})
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, name := range append(append([]string{}, loomNoiseFiles...), extraFiles...) {
		paths = append(paths, filepath.Join(home, filepath.FromSlash(name)))
	}
	p.files = newTestFiles(t, home, paths)
	learn := usage.NewMemory(fake.NewClock())
	for range 25 {
		for _, key := range []string{"cmd:Log in to Nevermind", "cmd:Log out of Nevermind"} {
			learn.Record(key, "lo")
			learn.Record(key, "loo")
		}
	}
	return withLearner(p, learn)
}

// TestLoomLeadsAndNoScatteredLettersAreListed is the reported bug: with Log in and Log out chosen for "lo" and "loo", the
// Loom app is first for "loom", and no row is listed for letters that only occur in its name.
func TestLoomLeadsAndNoScatteredLettersAreListed(t *testing.T) {
	items := rootResultsForTable(loomChosenFixture(t), "loom")
	if len(items) == 0 || items[0].Title != "Loom" || items[0].MatchTier() != search.TierExact {
		t.Fatalf("first row = %q, want the Loom application: %v", firstTitle(items), rowTitles2(items))
	}
	for _, it := range items {
		if it.Kind == search.KindFiles || strings.HasPrefix(it.Title, "Log ") {
			t.Errorf("%q is listed for \"loom\" by scattered letters", it.Title)
		}
	}
	for i, it := range items {
		if it.QueryFallback && i < len(items)-2 {
			t.Errorf("fallback %q is not among the last rows: %v", it.Title, rowTitles2(items))
		}
	}
}

// TestFilesNamedForTheQueryStillListBelowTheExactApp checks that dropping the scattered file hits keeps the files that do
// answer the query by name, after the exact app and before the query fallbacks.
func TestFilesNamedForTheQueryStillListBelowTheExactApp(t *testing.T) {
	items := rootResultsForTable(loomChosenFixture(t, "Documents/loom-recording.mp4", "Documents/Weekly Loom notes.md"), "loom")
	titles := rowTitles2(items)
	if titles[0] != "Loom" {
		t.Fatalf("rows = %v, want Loom first", titles)
	}
	for _, want := range []string{"loom-recording.mp4", "Weekly Loom notes.md"} {
		if indexOfItem(items, want) < 1 {
			t.Errorf("%q is not listed: %v", want, titles)
		}
	}
	for _, noise := range loomNoiseFiles {
		if indexOfItem(items, filepath.Base(noise)) >= 0 {
			t.Errorf("%q is listed for \"loom\": %v", noise, titles)
		}
	}
}

// TestInitialsStillFindACommandAtTheRoot checks that the word-start tier keeps initials after the scattered tier went.
func TestInitialsStillFindACommandAtTheRoot(t *testing.T) {
	items := rootResultsForTable(loomChosenFixture(t), "lit")
	if i := indexOfItem(items, "Log in to Nevermind"); i < 0 || items[i].MatchTier() != search.TierWordStart {
		t.Fatalf("\"lit\" does not list Log in to Nevermind by its initials: %v", rowTitles2(items))
	}
}
