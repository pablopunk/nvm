//go:build rig

package rigtest

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The installed-extension journeys run the extensions a real 1.x user converted (copies under
// ext/testdata/extensions/installed) in the real app under the rig, enabled through their review as a user does, and
// drive them in the palette. The rig has no macOS, so only what does not need Finder, osascript or the Mac's tools runs.

const installedExtensionRoot = "../ext/testdata/extensions/installed"

// seedInstalledExtension copies every file of one installed extension into the extensions folder of home.
func seedInstalledExtension(t *testing.T, home, id string) {
	t.Helper()
	src := filepath.Join(installedExtensionRoot, id)
	dst := filepath.Join(home, extensionsFolder, id)
	must(t, os.MkdirAll(dst, 0o755))
	entries, err := os.ReadDir(src)
	must(t, err)
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(src, entry.Name()))
		must(t, err)
		must(t, os.WriteFile(filepath.Join(dst, entry.Name()), data, 0o644))
	}
}

func installedPNG(t *testing.T) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 160, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 160; x++ {
			picture.Set(x, y, color.RGBA{R: uint8(x), G: uint8(2 * y), B: 160, A: 255})
		}
	}
	var buffer bytes.Buffer
	must(t, png.Encode(&buffer, picture))
	return buffer.Bytes()
}

// TestJourneyInstalledSnippetsOpensItsEmptyList enables the converted Snippets extension, opens it from the root and
// sees its empty state, as a user with no saved snippets does.
func TestJourneyInstalledSnippetsOpensItsEmptyList(t *testing.T) {
	home := t.TempDir()
	seedInstalledExtension(t, home, "local.text-snippets")
	app := Launch(t, Options{Home: home, Run: "snippets"})

	openExtensionsView(t, app)
	enableThroughReview(t, app, "snippets", "Snippets")
	hidePalette(t, app)

	must(t, app.SummonPalette())
	must(t, app.Type("Snippets"))
	waitForRowPrefix(t, app, "Snippets", elementWait)
	must(t, app.Key("Return"))
	_, err := app.WaitFor("No snippets yet", elementWait)
	must(t, err)
	shoot(t, app, "installed-snippets-empty")
}

// TestJourneyInstalledWallpapersGridShowsTheFolderImages enables the converted Wallpapers extension over a
// ~/.config/wallpapers folder with two pictures and sees both in its grid.
func TestJourneyInstalledWallpapersGridShowsTheFolderImages(t *testing.T) {
	home := t.TempDir()
	seedInstalledExtension(t, home, "local.wallpapers")
	folder := filepath.Join(home, ".config", "wallpapers")
	must(t, os.MkdirAll(folder, 0o755))
	for _, name := range []string{"a-forest.png", "b-dunes.png"} {
		must(t, os.WriteFile(filepath.Join(folder, name), installedPNG(t), 0o644))
	}
	app := Launch(t, Options{Home: home, Run: "wallpapers"})

	openExtensionsView(t, app)
	enableThroughReview(t, app, "wallpapers", "Wallpapers")
	hidePalette(t, app)

	must(t, app.SummonPalette())
	must(t, app.Type("Set Desktop Wallpaper"))
	waitForRowPrefix(t, app, "Set Desktop Wallpaper", elementWait)
	must(t, app.Key("Return"))
	_, err := app.WaitFor("a-forest.png", elementWait)
	must(t, err)
	_, err = app.WaitFor("b-dunes.png", elementWait)
	must(t, err)
	shoot(t, app, "installed-wallpapers-grid")
}

// TestJourneyInstalledExtensionsShowTheirOwnStatesWhereMacToolsAreMissing enables converted extensions that need macOS
// tools and opens each one in the real palette. On the Linux rig their tools are absent, so each must land on its own
// plain empty state, never a blank page or a raw error.
func TestJourneyInstalledExtensionsShowTheirOwnStatesWhereMacToolsAreMissing(t *testing.T) {
	home := t.TempDir()
	cases := []struct {
		id, title, row, query, expect, shot string
	}{
		{"local.quit-running-app", "Quit App", "Quit App", "kill app", "No Running Apps", "installed-quit-app"},
		{"local.restart-app", "Restart App", "Restart App", "relaunch app", "No Running Apps", "installed-restart-app"},
		{"local.remove-app-quarantine", "Remove App Quarantine", "Remove App Quarantine", "Remove App Quarantine", "No quarantined apps found", "installed-quarantine"},
	}
	for _, c := range cases {
		seedInstalledExtension(t, home, c.id)
	}
	app := Launch(t, Options{Home: home, Run: "states"})

	openExtensionsView(t, app)
	for _, c := range cases {
		enableThroughReview(t, app, c.title, c.title)
		clearSearch(t, app)
	}
	hidePalette(t, app)

	for _, c := range cases {
		must(t, app.SummonPalette())
		must(t, app.Type(c.query))
		waitForRowPrefix(t, app, c.row, elementWait)
		must(t, app.Key("Return"))
		_, err := app.WaitFor(c.expect, elementWait)
		must(t, err)
		shoot(t, app, c.shot)
		hidePalette(t, app)
	}
}

// TestJourneyInstalledSnippetsAddASnippetAndSeeItsActions walks the converted Snippets extension the way a user adds a
// first snippet: open the empty list, open the action menu, choose Add Snippet, type the text, save with the editor's
// shortcut, and find the new row with its copy, view, edit, paste and delete actions.
func TestJourneyInstalledSnippetsAddASnippetAndSeeItsActions(t *testing.T) {
	home := t.TempDir()
	seedInstalledExtension(t, home, "local.text-snippets")
	app := Launch(t, Options{Home: home, Run: "snippets-add"})

	openExtensionsView(t, app)
	enableThroughReview(t, app, "snippets", "Snippets")
	hidePalette(t, app)

	must(t, app.SummonPalette())
	must(t, app.Type("Snippets"))
	waitForRowPrefix(t, app, "Snippets", elementWait)
	must(t, app.Key("Return"))
	_, err := app.WaitFor("No snippets yet", elementWait)
	must(t, err)

	must(t, app.Key("ctrl+k"))
	_, err = app.WaitFor("Search actions", elementWait)
	must(t, err)
	waitForRow(t, app, "Add Snippet", elementWait)
	shoot(t, app, "installed-snippets-add-menu")
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Write the text you want to save…", elementWait)
	must(t, err)
	must(t, app.Type("Hello from the rig"))
	shoot(t, app, "installed-snippets-editor")
	must(t, app.Key("ctrl+Return"))
	waitForRow(t, app, "Hello from the rig", elementWait)
	shoot(t, app, "installed-snippets-saved")

	must(t, app.Key("ctrl+k"))
	_, err = app.WaitFor("Search actions", elementWait)
	must(t, err)
	for _, action := range []string{"View Snippet", "Edit Snippet", "Paste Snippet", "Delete Snippet"} {
		waitForRow(t, app, action, elementWait)
	}
	shoot(t, app, "installed-snippets-row-actions")
}
