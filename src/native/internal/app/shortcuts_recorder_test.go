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
