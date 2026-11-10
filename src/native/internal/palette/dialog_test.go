package palette

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// blockingDialog runs during while it is "open", as a native dialog blocks the caller until the user answers.
type blockingDialog struct {
	during func()
}

func (d blockingDialog) Choose(platform.ChooseRequest) ([]string, error) {
	d.during()
	return nil, nil
}

// TestFocusLossWhileADialogIsOpenKeepsThePalette checks the native dialog hold: the dialog takes the keyboard from the
// palette, and the focus loss it causes does not hide the palette. When the dialog closes the palette takes the keyboard
// back, and the next focus loss hides it.
func TestFocusLossWhileADialogIsOpenKeepsThePalette(t *testing.T) {
	var ran []string
	h := newHarness(t, rowsView("Root", &ran, "Alpha"), true)
	dialog := h.p.GuardDialog(blockingDialog{during: func() {
		blurOnce(h)
		passGrace(t, h)
	}})

	if _, err := dialog.Choose(platform.ChooseRequest{}); err != nil {
		t.Fatal(err)
	}
	if !h.win.IsVisible() {
		t.Fatal("the palette hid while its native dialog had the keyboard")
	}
	h.win.drain()
	if h.win.focusCalls != 1 {
		t.Fatalf("the palette took the keyboard back %d times after the dialog, want once", h.win.focusCalls)
	}

	blurOnce(h)
	passGrace(t, h)
	if h.win.IsVisible() {
		t.Fatal("a focus loss after the dialog closed should hide the palette")
	}
}

// TestNilDialogStaysNil checks a host without a dialog port: the guard must not turn the missing port into a dialog.
func TestNilDialogStaysNil(t *testing.T) {
	h := newHarness(t, rowsView("Root", new([]string), "Alpha"), true)
	if h.p.GuardDialog(nil) != nil {
		t.Fatal("GuardDialog(nil) should stay nil, so the views report the dialog unavailable")
	}
}
