//go:build rig

package rigtest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The extension journeys (J15, J16) drive the Extensions list the way a user does: summon, type the title, move
// down to the row, Return opens its review, Return on Enable turns it on. The rig reads the palette's labels only.

const (
	extensionFixtureRoot = "../ext/testdata/extensions"
	// extensionsFolder is the folder under the launch's HOME where the app keeps its extensions (configDir/extensions).
	extensionsFolder = ".config/nvm-native/extensions"
	escapePresses    = 4
)

// seedExtensionFolder copies one fixture into the extensions folder of home before launch, the way a user drops a
// folder in it. The folder is named after the id, which the host requires.
func seedExtensionFolder(t *testing.T, home, id string) {
	t.Helper()
	src := filepath.Join(extensionFixtureRoot, id)
	dst := filepath.Join(home, extensionsFolder, id)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"extension.json", "main.go"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		must(t, err)
		must(t, os.WriteFile(filepath.Join(dst, name), data, 0o644))
	}
}

// openExtensionsView summons the palette and opens the Extensions list from the root.
func openExtensionsView(t *testing.T, app *App) {
	t.Helper()
	must(t, app.SummonPalette())
	must(t, app.Type("extensions"))
	_, err := app.WaitFor("Manage and create extensions", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Reload Extensions", elementWait)
	must(t, err)
}

// chooseExtensionRow filters the Extensions list to query and moves to the row titled title. The list starts with
// the Create and Reload rows, so an extension's row is the third one.
func chooseExtensionRow(t *testing.T, app *App, query, title string) {
	t.Helper()
	must(t, app.Type(query))
	_, err := app.WaitFor(title, elementWait)
	must(t, err)
	must(t, app.Key("Down"))
	must(t, app.Key("Down"))
}

// clearSearch deletes the palette's whole query, one BackSpace per character, and waits until the field is empty.
// It does not assume where the query went after a view was popped.
func clearSearch(t *testing.T, app *App) {
	t.Helper()
	value, err := app.SearchValue()
	must(t, err)
	for range len(value) {
		must(t, app.Key("BackSpace"))
	}
	waitSearchValue(t, app, "")
}

// enableThroughReview opens the extension's review with Return and enables it with Return on the Enable row. The
// review is the only way to turn an extension on, so this is the path every journey uses.
func enableThroughReview(t *testing.T, app *App, query, title string) {
	t.Helper()
	chooseExtensionRow(t, app, query, title)
	must(t, app.Key("Return"))
	_, err := app.WaitFor("Review "+title, elementWait)
	must(t, err)
	must(t, app.Key("Return"))
}

// hidePalette presses Escape until the palette is hidden. Escape clears a query first, then leaves a pushed view,
// then hides the palette at the root.
func hidePalette(t *testing.T, app *App) {
	t.Helper()
	for range escapePresses {
		if visible, err := app.PaletteVisible(); err == nil && !visible {
			return
		}
		must(t, app.Key("Escape"))
		time.Sleep(pollInterval)
	}
	_, err := app.WaitPalette(hideWait, "hidden", isHidden)
	if err != nil {
		query, _ := app.SearchValue()
		t.Fatalf("palette still shown after %d Escape presses (query %q): %v", escapePresses, query, err)
	}
}

// assertNoRow fails when a palette row is labelled exactly label. Exact labels matter: the root also offers
// `Automate "alpha"`, and the search field's value holds the query, so substring matches would report those.
func assertNoRow(t *testing.T, app *App, label string) {
	t.Helper()
	tree, err := app.Tree()
	must(t, err)
	for _, node := range tree.Nodes {
		if node.Label == label {
			t.Fatalf("the palette shows a row labelled %q; it must not", label)
		}
	}
}

// waitForRow waits until a palette row is labelled exactly label.
func waitForRow(t *testing.T, app *App, label string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		tree, err := app.Tree()
		must(t, err)
		for _, node := range tree.Nodes {
			if node.Label == label {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no palette row labelled %q within %s; labels seen: %s", label, timeout, summarize(treeLabels(tree)))
		}
		time.Sleep(pollInterval)
	}
}
