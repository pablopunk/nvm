//go:build rig

package rigtest

import (
	"strings"
	"testing"
	"time"
)

// j15Extensions are the extensions J15 enables: the good one, then a child that panics, one that never answers, and
// one that answers with more rows than one protocol line can carry.
var j15Extensions = []struct{ id, query, title string }{
	{"acme.good", "good", "Good fixture"},
	{"acme.crash", "crash", "Crashing fixture"},
	{"acme.spin", "spin", "Hanging fixture"},
	{"acme.flood", "flood", "Flooding fixture"},
}

// TestJourneyJ15BrokenExtensionsDoNotStopThePalette is J15: a child that panics, one that hangs and one that floods
// its answer are enabled with the good one. Failed rows stay out of unrelated searches and appear, with their reason,
// only for a query that names the broken extension. The good extension keeps answering, and typing still updates the
// results on every keystroke.
// Not covered by the rig yet: the memory limit (acme.bomb), the restart and disable-after-repeated-failures path,
// and the ordering of the rows (the rig checks presence only).
func TestJourneyJ15BrokenExtensionsDoNotStopThePalette(t *testing.T) {
	home := t.TempDir()
	for _, ext := range j15Extensions {
		seedExtensionFolder(t, home, ext.id)
	}
	app := Launch(t, Options{Home: home})

	// One Extensions session for all four: once a broken extension runs, its failed row is the top hit of every root
	// search, so the root cannot be used to reopen the list.
	openExtensionsView(t, app)
	for _, ext := range j15Extensions {
		clearSearch(t, app)
		enableThroughReview(t, app, ext.query, ext.title)
	}
	hidePalette(t, app)

	must(t, app.SummonPalette())
	started := time.Now()
	must(t, app.Type("alpha"))
	waitForRow(t, app, "alpha", clipWait)
	t.Logf("good row shown after typing alpha, with broken extensions enabled: %s", time.Since(started).Round(time.Millisecond))

	assertNoLabelContaining(t, app, "is not responding")
	shoot(t, app, "j15-1-good-row-without-failed-rows")

	// A failed extension shows its pointer row only when the query names it. The crashing one's reason is its panic;
	// the hanging and flooding ones are checked by title, because their subtitle is the reason of the first failure.
	reasons := map[string]string{"acme.crash": "panic: crash fixture"}
	for _, ext := range j15Extensions[1:] {
		clearSearch(t, app)
		must(t, app.Type(ext.query))
		_, err := app.WaitFor(ext.title+" is not responding", elementWait)
		must(t, err)
		if reason, ok := reasons[ext.id]; ok {
			_, err = app.WaitFor(reason, elementWait)
			must(t, err)
		}
	}
	shoot(t, app, "j15-2-named-failed-rows-with-reasons")
	clearSearch(t, app)
	must(t, app.Type("alpha"))
	waitForRow(t, app, "alpha", elementWait)

	must(t, app.Type("zzz"))
	waitSearchValue(t, app, "alphazzz")
	assertNoRow(t, app, "alpha")
	shoot(t, app, "j15-3-typing-updates-while-broken")

	for range 3 {
		must(t, app.Key("BackSpace"))
	}
	waitSearchValue(t, app, "alpha")
	waitForRow(t, app, "alpha", elementWait)
	shoot(t, app, "j15-4-typing-restores-good-row")

	if app.isExited() {
		t.Fatalf("the app exited during J15; see %s", app.logPath)
	}
	clearSearch(t, app)
	must(t, app.DismissWithEscape())
}

// assertNoLabelContaining fails when any palette label contains text. Failed rows carry the extension's title in their
// label, so the check finds them for any extension.
func assertNoLabelContaining(t *testing.T, app *App, text string) {
	t.Helper()
	tree, err := app.Tree()
	must(t, err)
	for _, node := range tree.Nodes {
		if strings.Contains(node.Label, text) {
			t.Fatalf("the palette shows a row labelled %q; it must not for this query", node.Label)
		}
	}
}
