//go:build rig

package rigtest

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	// recoveryHotkey is the first Linux recovery accelerator; the banner names it.
	recoveryHotkey = "Control+Alt+K"
	// bannerWait covers the background retries of the configured shortcut, about three seconds, before the
	// banner and the launch-time palette appear.
	bannerWait = 15 * time.Second
	// The palette is centered on its display; a couple of pixels of slack cover rounding.
	centeringSlackX = 2
)

// TestJourneyJ11TakenHotkeyShowsTheBannerAndTheRecoveryShortcut is the conflict path of J11 and the hotkey notice
// of J25. A second copy on the same display cannot bind Alt+Shift+Space, so it opens once at launch with a banner
// that names the recovery shortcut, opens with that shortcut, and places its palette centered on the display. The
// first copy keeps its own shortcut. Saving a taken shortcut from the palette is not covered yet.
func TestJourneyJ11TakenHotkeyShowsTheBannerAndTheRecoveryShortcut(t *testing.T) {
	first := Launch(t, Options{Run: "first"})
	second := Launch(t, Options{Run: "second", AllowUnboundHotkey: true})
	_, err := second.WaitFor(DefaultHotkey+" is unavailable", bannerWait)
	must(t, err)
	_, err = second.WaitFor("Use "+recoveryHotkey+" to reopen Nevermind.", elementWait)
	must(t, err)
	shoot(t, second, "j11-1-banner-at-launch")

	must(t, second.DismissWithEscape())
	second.hotkey = recoveryHotkey
	must(t, second.SummonPalette())
	must(t, second.Type("clipboard"))
	_, err = second.WaitFor("Clipboard History", elementWait)
	must(t, err)
	shoot(t, second, "j11-2-recovery-shortcut-opens")

	must(t, assertPaletteCentered(second))
	must(t, second.DismissWithEscape())

	must(t, first.SummonPalette())
	shoot(t, first, "j11-3-first-copy-keeps-its-shortcut")
	must(t, first.DismissWithEscape())
}

// assertPaletteCentered checks that the palette's horizontal center is the display's, within centeringSlackX.
func assertPaletteCentered(app *App) error {
	w, err := app.PaletteWindow()
	if err != nil {
		return err
	}
	width, err := displayWidth(app)
	if err != nil {
		return err
	}
	center := w.Bounds.X + w.Bounds.Width/2
	if math.Abs(center-width/2) > centeringSlackX {
		return fmt.Errorf("palette center x = %.0f on a %.0f px display; want the middle (%d px slack)", center, width, centeringSlackX)
	}
	return nil
}

// displayWidth reads the width of the app's display with xdotool.
func displayWidth(app *App) (float64, error) {
	out, err := app.output("xdotool", "getdisplaygeometry")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(out)
	if len(fields) < 1 {
		return 0, fmt.Errorf("xdotool getdisplaygeometry printed %q", out)
	}
	return strconv.ParseFloat(fields[0], 64)
}
