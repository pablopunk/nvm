package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TestFooterHintsSpellEscapeAndAcceleratorsAsThePaletteDoes: Escape is "esc", an accelerator is spelled as shortcuts
// are, and the hints are three spaces apart.
func TestFooterHintsSpellEscapeAndAcceleratorsAsThePaletteDoes(t *testing.T) {
	hints := []api.FooterHint{{Key: "Escape", Title: "Back"}, {Key: "Command+N", Title: "New"}, {Key: "Command+O", Title: "All notes"}}
	want := "esc Back   " + shortcutLabel("Command+N", "") + " New   " + shortcutLabel("Command+O", "") + " All notes"
	if got := footerHintsText(hints, ""); got != want {
		t.Fatalf("footerHintsText = %q; want %q", got, want)
	}
}
