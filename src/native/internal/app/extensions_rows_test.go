package app

import (
	"context"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/search"
)

// collectExtensionRows runs the root's extension search for q on a real manager and returns every row it emits.
func collectExtensionRows(t *testing.T, p *App, q string) []search.Item {
	t.Helper()
	var rows []search.Item
	extensionProvider(p, &rootInput{}).Search(context.Background(), q, func(batch search.Batch) {
		rows = append(rows, batch.Items...)
	})
	return rows
}

func TestFailedExtensionShowsAPointerRowOnlyWhenTheQueryNamesIt(t *testing.T) {
	m := newFixtureManager(t, "acme.syntax")
	waitForState(t, m, "acme.syntax", supervisor.StateBroken)
	p := &App{}
	p.exts.mgr.Store(m)

	for _, row := range collectExtensionRows(t, p, "alpha") {
		if row.Accessory == "Failed" {
			t.Fatalf("query that does not name the extension showed failed row %q", row.Title)
		}
	}
	row := findRowTitled(t, collectExtensionRows(t, p, "syntax"), "Broken source fixture is not responding")
	if row.Accessory != "Failed" || !strings.Contains(row.Subtitle, "does not load") {
		t.Fatalf("failed row = accessory %q subtitle %q, want Failed with the reason", row.Accessory, row.Subtitle)
	}
}

func TestFailedExtensionNeverShowsAtTheRootOrAboveRealRows(t *testing.T) {
	m := newFixtureManager(t, "acme.syntax")
	waitForState(t, m, "acme.syntax", supervisor.StateBroken)
	p := &App{}
	p.exts.mgr.Store(m)

	for _, row := range extensionProvider(p, &rootInput{}).Root(context.Background()) {
		if row.Accessory == "Failed" || row.Title == "Broken source fixture is not responding" {
			t.Fatalf("the root shows the failed extension as %q", row.Title)
		}
	}
	pointer := findRowTitled(t, collectExtensionRows(t, p, "syntax"), "Broken source fixture is not responding")
	if pointer.Score >= scoreQueryFallback {
		t.Fatalf("failed row score %d must rank below the query fallback band (%d)", pointer.Score, scoreQueryFallback)
	}
}

func TestRunningExtensionYieldsItsItemRows(t *testing.T) {
	m := newFixtureManager(t, "acme.good")
	waitForState(t, m, "acme.good", supervisor.StateRunning)
	p := &App{}
	p.exts.mgr.Store(m)

	rows := collectExtensionRows(t, p, "alpha")
	item := findRowTitled(t, rows, "alpha")
	if item.Kind != "Good fixture" || item.Accessory != "" {
		t.Fatalf("item row = kind %q accessory %q, want the extension's item", item.Kind, item.Accessory)
	}
	for _, row := range rows {
		if row.Accessory == "Failed" {
			t.Fatalf("running extension yielded a failed row %q", row.Title)
		}
	}
}

// pendingCommandDeps starts a command's invocation and never answers it, so the palette is observed while the run is
// still in flight: whatever has closed it by then was the dispatch-time dismissal, not the result.
func pendingCommandDeps() extview.Deps {
	return extview.Deps{Invoke: func(string, api.Invocation, func(api.Result, *api.Failure)) {}}
}

// commandLeavesPaletteOpen runs a command row's action through the palette and reports whether the palette is still shown.
func commandLeavesPaletteOpen(t *testing.T, command api.Command) bool {
	t.Helper()
	p := shownPalette(t)
	p.RunAction(extview.ActionFor(pendingCommandDeps(), "acme.commands", commandAction(command)))
	return p.Visible()
}

func TestCommandThatDismissesAfterRunClosesThePalette(t *testing.T) {
	if commandLeavesPaletteOpen(t, api.Command{ID: "run", Title: "Run", DismissAfterRun: true}) {
		t.Fatal("a command with DismissAfterRun left the palette open after its run")
	}
}

func TestCommandWithoutDismissAfterRunKeepsThePaletteOpen(t *testing.T) {
	if !commandLeavesPaletteOpen(t, api.Command{ID: "run", Title: "Run"}) {
		t.Fatal("a command without DismissAfterRun closed the palette after its run")
	}
}
