//go:build rig

package rigtest

import (
	"bytes"
	"image"
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
	var buffer bytes.Buffer
	must(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 16, 9))))
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
// plain empty state or sign-in form, never a blank page or a raw error.
func TestJourneyInstalledExtensionsShowTheirOwnStatesWhereMacToolsAreMissing(t *testing.T) {
	home := t.TempDir()
	cases := []struct {
		id, title, query, expect, shot string
	}{
		{"local.quit-running-app", "Quit App", "kill app", "No Running Apps", "installed-quit-app"},
		{"local.restart-app", "Restart App", "relaunch app", "No Running Apps", "installed-restart-app"},
		{"local.remove-app-quarantine", "Remove App Quarantine", "Remove App Quarantine", "No quarantined apps found", "installed-quarantine"},
		{"local.paperless-ai-search", "Paperless AI Search", "Notflix Documents", "Paperless API Token", "installed-paperless-connect"},
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
		waitForRowPrefix(t, app, c.title, elementWait)
		must(t, app.Key("Return"))
		_, err := app.WaitFor(c.expect, elementWait)
		must(t, err)
		shoot(t, app, c.shot)
		hidePalette(t, app)
	}
}
