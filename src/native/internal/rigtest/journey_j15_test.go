//go:build rig

package rigtest

import (
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
// its answer are enabled with the good one. Each broken extension shows a failed row with its reason, the good
// extension keeps answering, and typing still updates the results on every keystroke.
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

	for _, title := range []string{"Crashing fixture is not responding", "Hanging fixture is not responding", "Flooding fixture is not responding"} {
		_, err := app.WaitFor(title, elementWait)
		must(t, err)
	}
	_, err := app.WaitFor("panic: crash fixture", elementWait)
	must(t, err)
	// After its first failure a child answers "not running: failed", so that is the reason the palette shows for the
	// hanging and flooding extensions. The first failure's own reason (no answer in time, line limit) is not kept.
	_, err = app.WaitFor("not running: failed", elementWait)
	must(t, err)
	shoot(t, app, "j15-1-good-row-and-failed-rows")

	must(t, app.Type("zzz"))
	waitSearchValue(t, app, "alphazzz")
	assertNoRow(t, app, "alpha")
	shoot(t, app, "j15-2-typing-updates-while-broken")

	for range 3 {
		must(t, app.Key("BackSpace"))
	}
	waitSearchValue(t, app, "alpha")
	waitForRow(t, app, "alpha", elementWait)
	shoot(t, app, "j15-3-typing-restores-good-row")

	if app.isExited() {
		t.Fatalf("the app exited during J15; see %s", app.logPath)
	}
	clearSearch(t, app)
	must(t, app.DismissWithEscape())
}
