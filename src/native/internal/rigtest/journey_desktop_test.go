//go:build rig

package rigtest

import (
	"strings"
	"testing"
	"time"
)

// desktopRowPrefixes are the rows the desktop fixture shows whose text depends on the desktop: the rig fixes neither
// the window count nor the palette accelerator's spelling, so these rows are matched by their prefix.
var desktopRowPrefixes = []struct{ query, prefix string }{
	{"apps", "apps listed=true"},
	{"shortcuts", "shortcuts palette="},
	{"system", "system os=linux"},
	{"history", "history entries="},
	{"ocr", "ocr refused=true"},
	{"files", "files refused=true"},
	{"links", "links refused=true"},
}

// TestJourneyDesktopFixtureWritesTheClipboardAndListsWindows enables the desktop fixture and the undeclared fixture.
// The desktop fixture writes the clipboard through the host, and the test reads it back from the real selection. It
// lists the windows of a probe window the test starts first. The undeclared fixture shows every probe refused.
func TestJourneyDesktopFixtureWritesTheClipboardAndListsWindows(t *testing.T) {
	home := t.TempDir()
	seedExtensionFolder(t, home, "acme.desktop")
	seedExtensionFolder(t, home, "acme.undeclared")
	startTargetWindow(t)
	app := Launch(t, Options{Home: home})

	openExtensionsView(t, app)
	clearSearch(t, app)
	enableThroughReview(t, app, "desktop", "Desktop fixture")
	clearSearch(t, app)
	enableThroughReview(t, app, "undeclared", "Undeclared fixture")
	hidePalette(t, app)

	must(t, app.SummonPalette())
	clearSearch(t, app)
	must(t, app.Type("clipboard"))
	waitForRow(t, app, "clipboard read=nvm desktop fixture", elementWait)
	if got, err := app.ClipboardGet(); err != nil || got != "nvm desktop fixture" {
		t.Fatalf("the clipboard holds %q (%v), want the fixture's text", got, err)
	}

	clearSearch(t, app)
	must(t, app.Type("windows"))
	if label := waitForRowPrefix(t, app, "windows listed=", elementWait); strings.HasPrefix(label, "windows listed=0") {
		t.Fatalf("the probe window is not listed: %q", label)
	}

	for _, check := range desktopRowPrefixes {
		clearSearch(t, app)
		must(t, app.Type(check.query))
		waitForRowPrefix(t, app, check.prefix, elementWait)
	}

	clearSearch(t, app)
	must(t, app.Type("undeclared"))
	waitForRow(t, app, "undeclared refused=7 of 7", elementWait)
	shoot(t, app, "desktop-fixture-rows")
}

// waitForRowPrefix waits for a palette row whose label starts with prefix and returns that label.
func waitForRowPrefix(t *testing.T, app *App, prefix string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		tree, err := app.Tree()
		must(t, err)
		for _, node := range tree.Nodes {
			if strings.HasPrefix(node.Label, prefix) {
				return node.Label
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no palette row starting with %q within %s", prefix, timeout)
		}
		time.Sleep(pollInterval)
	}
}
