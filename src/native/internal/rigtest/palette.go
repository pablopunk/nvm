//go:build rig

package rigtest

import (
	"fmt"
	"time"
)

const (
	// DefaultHotkey is the app's palette hotkey when Options.Hotkey is empty.
	DefaultHotkey = "Alt+Shift+Space"

	summonWait    = 3 * time.Second
	summonPresses = 2
	hideWait      = 3 * time.Second
	// The bottom-left corner of the 1280x800 rig display, far from the centered palette.
	clickAwayX, clickAwayY = 40, 760
)

func isVisible(w Window) bool { return w.Visible }

func isVisibleAndFocused(w Window) bool { return w.Visible && w.Focused }

func isHidden(w Window) bool { return !w.Visible }

// PaletteVisible reports whether the palette window is shown.
func (a *App) PaletteVisible() (bool, error) {
	w, err := a.PaletteWindow()
	return w.Visible, err
}

// SummonPalette shows the palette with the hotkey and waits until it is visible and focused.
//
// Workaround, encoded here and not in the tests: right after "ready" the first press of the hotkey
// can be lost (another client may still hold the grab, or the key arrives before the app binds it).
// While the palette is still hidden after summonWait, the press is sent again. A press is only
// repeated while hidden, because a press on a shown palette hides it.
func (a *App) SummonPalette() error {
	for press := 1; press <= summonPresses; press++ {
		if visible, err := a.PaletteVisible(); err == nil && visible {
			break
		}
		if err := a.PressHotkey(a.hotkey); err != nil {
			return err
		}
		if _, err := a.WaitPalette(summonWait, "visible", isVisible); err == nil {
			break
		}
		a.t.Logf("hotkey press %d did not show the palette within %s; retrying while hidden", press, summonWait)
	}
	_, err := a.WaitPalette(summonWait, "visible and focused", isVisibleAndFocused)
	return err
}

// DismissWithEscape presses Escape and waits for the palette to hide. Escape at the root with an
// empty query hides it; with a query it clears the query first, so callers clear the query first.
func (a *App) DismissWithEscape() error {
	if err := a.Key("Escape"); err != nil {
		return err
	}
	_, err := a.WaitPalette(hideWait, "hidden", isHidden)
	return err
}

// ClickAway clicks the desktop outside the palette, which must hide it (the window loses focus).
func (a *App) ClickAway() error {
	if err := a.Click(clickAwayX, clickAwayY); err != nil {
		return err
	}
	_, err := a.WaitPalette(hideWait, "hidden", isHidden)
	return err
}

// SearchValue is the text of the palette's search field, as assistive technology reads it.
func (a *App) SearchValue() (string, error) {
	node, err := a.FindNode("textField", "Search")
	if err != nil {
		return "", fmt.Errorf("search field: %w", err)
	}
	return node.Value, nil
}
