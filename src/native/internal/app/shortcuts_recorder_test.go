package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// Control and Alt held with the Left arrow are one chord; the recorder must not drop either modifier on the way.
func TestTheRecorderKeepsControlAndAltOfALeftArrowChord(t *testing.T) {
	mods := recordedModifiers(ui.Ctrl | ui.Alt)
	if got := shortcuts.Capture(mods, palette.KeyName(ui.KeyLeft)); got != "Control+Alt+Left" {
		t.Fatalf("recorded %q, want Control+Alt+Left", got)
	}
}

// TestRecorderKeepsItsPromptWhileAModifierIsHeld checks that Control held when a plain recorder is open does not replace
// its prompt with the held keys, as 1.x keeps the prompt until a key press records a shortcut.
func TestRecorderKeepsItsPromptWhileAModifierIsHeld(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.shortcutRecorderView(clipboardEntry))
	tt := ui.NewTester(p.View, width, height)
	tt.HoldModifiers(ui.Ctrl)
	tt.Frame()
	if _, ok := tt.Find("Press a keyboard shortcut"); !ok {
		t.Fatal("a held Control replaced the recorder's prompt")
	}
}
