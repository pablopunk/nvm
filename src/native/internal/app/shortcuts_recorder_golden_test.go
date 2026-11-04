package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
)

// holdingInteraction holds the modifiers before the frame is compared, so the recorder shows them as keys.
func holdingInteraction(mods ui.Modifiers) func(*ui.Tester) {
	return func(tt *ui.Tester) { tt.HoldModifiers(mods) }
}

// TestPaletteShortcutRecorderHeldModifiers is the recorder with Super and Alt held and no key yet.
func TestPaletteShortcutRecorderHeldModifiers(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.shortcutRecorderView(clipboardEntry))
	held := paletteGoldens
	held.Interact = holdingInteraction(ui.Super | ui.Alt)
	golden.AssertMatrix(t, "shortcuts-recorder-held", p.View, held)
}

// TestPaletteShortcutRecorderHyperCap records Hyper+D, the four Hyper modifiers and D: the Hyper key shows as one ✦ cap.
func TestPaletteShortcutRecorderHyperCap(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.shortcutRecorderView(clipboardEntry))
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(ui.Super|ui.Ctrl|ui.Alt|ui.Shift, ui.KeyD)
	tt.Frame()
	golden.AssertMatrix(t, "shortcuts-recorder-hyper-cap", p.View, paletteGoldens)
}

// TestPaletteHyperRecorderHeldModifiers is the Hyper key recorder with the four Hyper modifiers held. It spells them out:
// the Hyper key being set is not yet a ✦ shortcut.
func TestPaletteHyperRecorderHeldModifiers(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.hyperRecorderView())
	hyper := paletteGoldens
	hyper.Interact = holdingInteraction(ui.Super | ui.Ctrl | ui.Alt | ui.Shift)
	golden.AssertMatrix(t, "hyper-recorder-held", p.View, hyper)
}
