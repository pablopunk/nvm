package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search/usage"
)

// paletteWithLearnFixture is the golden palette whose extension source is the running acme.learn fixture, with a
// usage memory that the test fills.
func paletteWithLearnFixture(t *testing.T, learn *usage.Learner) *App {
	t.Helper()
	m := newFixtureManager(t, "acme.learn")
	waitForState(t, m, "acme.learn", supervisor.StateRunning)
	p := withLearner(goldenPalette(t), learn)
	p.exts.mgr.Store(m)
	return p
}

// TestFolderExtensionRowClimbsWhenItsKeyIsUsed records a use of the longer-titled row's learning key and expects that
// row to rank above the other, which matches as well and has no use. Unused rows tie by shorter title.
func TestFolderExtensionRowClimbsWhenItsKeyIsUsed(t *testing.T) {
	learn := usage.NewMemory(fake.NewClock())
	p := paletteWithLearnFixture(t, learn)
	before := rootResultsForTable(p, "learn")
	if indexOfItem(before, "Learn Alpha") < 0 || indexOfItem(before, "Learn Beta") < 0 {
		t.Fatalf("the fixture rows are missing, got %v", rowTitlesOf(before))
	}
	if indexOfItem(before, "Learn Beta") > indexOfItem(before, "Learn Alpha") {
		t.Fatalf("unused rows must tie by shorter title, got %v", rowTitles(p))
	}
	learn.Record("ext:acme.learn:alpha", "learn")
	after := rootResultsForTable(p, "learn")
	if indexOfItem(after, "Learn Alpha") > indexOfItem(after, "Learn Beta") {
		t.Fatalf("the used row must climb above the unused one, got %v", rowTitles(p))
	}
}

// TestFolderExtensionLearningKeysAreNamespacedByTheExtension records the bare key, which a built-in or another
// extension would use, and expects no boost for the folder row.
func TestFolderExtensionLearningKeysAreNamespacedByTheExtension(t *testing.T) {
	learn := usage.NewMemory(fake.NewClock())
	p := paletteWithLearnFixture(t, learn)
	learn.Record("alpha", "learn")
	rows := rootResultsForTable(p, "learn")
	if indexOfItem(rows, "Learn Beta") > indexOfItem(rows, "Learn Alpha") {
		t.Fatalf("a bare key must not boost a folder row, got %v", rowTitles(p))
	}
}
