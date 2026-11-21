package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// TestPaletteHotkeyBannerWhenTakenWithRecovery is the banner for a shortcut another app holds on Linux: it names
// the recovery shortcut that still opens the palette.
func TestPaletteHotkeyBannerWhenTakenWithRecovery(t *testing.T) {
	p := goldenPalette(t)
	p.showSummonNotice(shortcuts.Status{Accelerator: "Alt+Shift+Space", Recovery: "Control+Alt+K"})
	golden.AssertMatrix(t, "hotkey-taken", p.View, paletteGoldens)
}

// TestPaletteHotkeyBannerWhenNoShortcutIsSet is the banner of a palette with no summon shortcut.
func TestPaletteHotkeyBannerWhenNoShortcutIsSet(t *testing.T) {
	p := goldenPalette(t)
	p.showSummonNotice(shortcuts.Status{})
	golden.AssertMatrix(t, "hotkey-unset", p.View, paletteGoldens)
}
