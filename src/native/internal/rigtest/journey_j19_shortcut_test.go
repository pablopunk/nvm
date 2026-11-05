//go:build rig

package rigtest

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestJourneyJ19ShortcutAppliesLayoutOnX11 binds Left Half to Control+Shift+Left from its Command-K menu, as 1.x binds a
// window command: the Keyboard Shortcuts list shows a command once it has a shortcut, so the list is checked after the
// binding. Then it presses the shortcut with the probe focused: the probe reaches the left half, and the palette's focus
// return leaves the probe in front. Not covered: the recorder's conflict messages, macOS, and any layout other than Left
// Half.
func TestJourneyJ19ShortcutAppliesLayoutOnX11(t *testing.T) {
	const accelerator = "Control+Shift+Left"
	probeID := startProbe(t)
	must(t, exec.Command("xdotool", "windowactivate", "--sync", probeID).Run())

	app := Launch(t, Options{})
	must(t, app.SummonPalette())
	must(t, app.Type("left half"))
	_, err := app.WaitFor("Left Half", elementWait)
	must(t, err)
	must(t, app.Key(actionMenuKey))
	_, err = app.WaitFor("Set keyboard shortcut", elementWait)
	must(t, err)
	must(t, app.Key("Down")) // the menu lists the run action, then the settings, then Set keyboard shortcut
	must(t, app.Key("Down"))
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Press a keyboard shortcut", elementWait)
	must(t, err)

	must(t, app.PressHotkey(accelerator))
	_, err = app.WaitFor("Save shortcut for “Left Half”", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	must(t, app.Key("Escape"))
	must(t, app.DismissWithEscape())

	must(t, app.SummonPalette())
	must(t, app.Type("keyboard shortcuts"))
	_, err = app.WaitFor("Keyboard Shortcuts", elementWait)
	must(t, err)
	must(t, app.Key("Return"))
	_, err = app.WaitFor("Search Keyboard Shortcuts", elementWait)
	must(t, err)
	must(t, app.Type("left half"))
	_, err = app.WaitFor("Left Half", elementWait)
	must(t, err)
	shoot(t, app, "j19s-1-bound")
	must(t, app.Key("Escape"))
	must(t, app.DismissWithEscape())
	waitActiveWindow(t, probeID)

	must(t, app.PressHotkey(accelerator))
	waitProbeFrame(t, probeID, leftHalfOfWorkArea(t))
	shoot(t, app, "j19s-2-applied")
}

// waitActiveWindow waits until the X11 active window is id, which the palette's focus return must leave in place.
func waitActiveWindow(t *testing.T, id string) {
	t.Helper()
	for elapsed := time.Duration(0); elapsed < elementWait; elapsed += pollInterval {
		out, err := exec.Command("xdotool", "getactivewindow").Output()
		if err == nil && strings.TrimSpace(string(out)) == id {
			return
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("the probe %s did not get the keyboard back after the palette hid", id)
}
