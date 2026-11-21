//go:build rig

package rigtest

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// targetWallpaper is the image the target window shows; it is the rig's fixed wallpaper.
const targetWallpaper = "../../scripts/rig/assets/wall.png"

// TestJourneyJ01EscapeAndTheShortcutReturnFocusToTheWindowBeforeTheSummon is the focus part of J01: the palette is
// summoned over a window, and hiding it with Escape or with the shortcut gives the keyboard back to that window.
// The check reads the active window with xdotool. Focus after a click away is not covered yet.
func TestJourneyJ01EscapeAndTheShortcutReturnFocusToTheWindowBeforeTheSummon(t *testing.T) {
	target := startTargetWindow(t)
	app := Launch(t, Options{})

	must(t, app.SummonPalette())
	must(t, app.DismissWithEscape())
	assertActiveWindow(t, target)

	must(t, app.SummonPalette())
	must(t, app.PressHotkey(DefaultHotkey))
	_, err := app.WaitPalette(hideWait, "hidden", isHidden)
	must(t, err)
	assertActiveWindow(t, target)
	shoot(t, app, "j01-focus-returned")
}

// startTargetWindow opens an image window and makes it the active window, as the app the user works in would be.
// It is closed at cleanup.
func startTargetWindow(t *testing.T) string {
	t.Helper()
	display := os.Getenv("DISPLAY")
	cmd := exec.Command("feh", "--title", "rig-target", "--geometry", "400x300+100+100", targetWallpaper)
	cmd.Env = withDisplay(display)
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the target window: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	id := waitForWindowID(t, display, "rig-target")
	if _, err := xdotoolOn(display, "windowactivate", "--sync", id); err != nil {
		t.Fatalf("activating the target window: %v", err)
	}
	return id
}

// waitForWindowID polls xdotool until a window titled title exists, and returns its ID.
func waitForWindowID(t *testing.T, display, title string) string {
	t.Helper()
	deadline := time.Now().Add(elementWait)
	for {
		out, err := xdotoolOn(display, "search", "--name", "^"+title+"$")
		if err == nil {
			if ids := strings.Fields(out); len(ids) > 0 {
				return ids[0]
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no window titled %q appeared", title)
		}
		time.Sleep(pollInterval)
	}
}

// assertActiveWindow fails the test unless the keyboard is with the window id.
func assertActiveWindow(t *testing.T, id string) {
	t.Helper()
	deadline := time.Now().Add(hideWait)
	var active string
	for {
		out, err := xdotoolOn(os.Getenv("DISPLAY"), "getactivewindow")
		active = strings.TrimSpace(out)
		if err == nil && active == id {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the active window is %q, want the window that had the keyboard before the summon (%s)", active, id)
		}
		time.Sleep(pollInterval)
	}
}

// xdotoolOn runs xdotool on display and returns its standard output.
func xdotoolOn(display string, args ...string) (string, error) {
	cmd := exec.Command("xdotool", args...)
	cmd.Env = withDisplay(display)
	out, err := cmd.Output()
	return string(out), err
}
