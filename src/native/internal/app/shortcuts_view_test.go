package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

var clipboardEntry = shortcuts.Entry{ActionID: shortcutCommandID("Clipboard History"), Title: "Clipboard History"}

// goldenShortcutsList is the Keyboard Shortcuts list over the root: Clipboard History has a shortcut, and Quick Notes
// asked for one the OS refused.
func goldenShortcutsList(t *testing.T) *App {
	t.Helper()
	p := goldenPalette(t)
	manager := p.shortcutsManager()
	mustSetShortcut(t, manager, shortcutCommandID("Clipboard History"), "Command+Alt+V")
	p.ports.Hotkeys.(*fake.Hotkeys).FailNext(fake.OpHotkeysRegister, platform.ErrConflict)
	mustSetShortcut(t, manager, shortcutCommandID("Quick Notes"), "Command+Alt+N")
	goldenOpen(p, p.keyboardShortcutsView())
	return p
}

func mustSetShortcut(t *testing.T, manager *shortcuts.Manager, actionID, accelerator string) {
	t.Helper()
	if err := manager.SetAction(actionID, accelerator); err != nil {
		t.Fatalf("SetAction(%s): %v", actionID, err)
	}
}

func entryOf(t *testing.T, p *App, actionID string) shortcuts.Entry {
	t.Helper()
	for _, entry := range p.shortcutsManager().Entries() {
		if entry.ActionID == actionID {
			return entry
		}
	}
	t.Fatalf("no entry for %s", actionID)
	return shortcuts.Entry{}
}

// TestKeyboardShortcutsListShowsEachShortcutAndItsRefusal searches the list and reads the rows it draws.
func TestKeyboardShortcutsListShowsEachShortcutAndItsRefusal(t *testing.T) {
	p := goldenShortcutsList(t)
	rows := p.shortcutRows("")
	byTitle := map[string]string{}
	for _, row := range rows {
		byTitle[row.Title] = row.Accessory + "|" + row.Subtitle
	}
	if got := byTitle["Palette Hotkey"]; got != shortcutLabelOf("Alt+Shift+Space")+"|" {
		t.Fatalf("palette row = %q", got)
	}
	if got := byTitle["Clipboard History"]; got != shortcutLabelOf("Command+Alt+V")+"|" {
		t.Fatalf("clipboard row = %q", got)
	}
	if got := byTitle["Quick Notes"]; got != shortcutLabelOf("Command+Alt+N")+"|Not active: Command+Alt+N is in use by another app" {
		t.Fatalf("notes row = %q", got)
	}
	if got := len(p.shortcutRows("clip")); got != 1 {
		t.Fatalf("search for clip found %d rows, want 1", got)
	}
}

// TestRecorderSavesThePressedShortcutOnEnter presses Command+Alt+V in the recorder and saves it with Enter.
func TestRecorderSavesThePressedShortcutOnEnter(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.shortcutRecorderView(clipboardEntry))
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(ui.Super|ui.Alt, ui.KeyV)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := entryOf(t, p, clipboardEntry.ActionID).Accelerator; got != "Command+Alt+V" {
		t.Fatalf("saved accelerator = %q, want Command+Alt+V", got)
	}
	if p.shortcutsManager().Suspended() {
		t.Fatal("saving left the bindings released")
	}
}

// TestRecorderEscapeGoesBackAndKeepsTheShortcut records a key, then leaves with Escape: nothing is saved, and the
// bindings come back.
func TestRecorderEscapeGoesBackAndKeepsTheShortcut(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.shortcutRecorderView(clipboardEntry))
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(ui.Super|ui.Alt, ui.KeyV)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if got := entryOf(t, p, clipboardEntry.ActionID).Accelerator; got != "" {
		t.Fatalf("escape saved %q", got)
	}
	if p.shortcutsManager().Suspended() {
		t.Fatal("escape left the bindings released")
	}
}

// TestRecorderNeedsAModifierBeforeItSaves presses a bare letter, which records nothing, and then Enter.
func TestRecorderNeedsAModifierBeforeItSaves(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.shortcutRecorderView(clipboardEntry))
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(0, ui.KeyV)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := entryOf(t, p, clipboardEntry.ActionID).Accelerator; got != "" {
		t.Fatalf("a bare key saved %q", got)
	}
}

// TestPaletteRecorderChangesThePaletteHotkey records Control+Shift+Space as the palette key.
func TestPaletteRecorderChangesThePaletteHotkey(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.paletteRecorderView())
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(ui.Ctrl|ui.Shift, ui.KeySpace)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := p.shortcutsManager().PaletteAccelerator(); got != "Control+Shift+Space" {
		t.Fatalf("palette = %q, want Control+Shift+Space", got)
	}
}

// The goldens of the Keyboard Shortcuts list and of the recorder, empty and with a key pressed.
func TestPaletteShortcutsList(t *testing.T) {
	p := goldenShortcutsList(t)
	golden.AssertMatrix(t, "shortcuts-list", p.View, paletteGoldens)
}

func TestPaletteShortcutRecorderEmpty(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.shortcutRecorderView(clipboardEntry))
	golden.AssertMatrix(t, "shortcuts-recorder", p.View, paletteGoldens)
}

func TestPaletteShortcutRecorderRecorded(t *testing.T) {
	p := goldenPalette(t)
	goldenOpen(p, p.shortcutRecorderView(clipboardEntry))
	tt := ui.NewTester(p.View, width, height)
	tt.Frame()
	tt.Key(ui.Super|ui.Alt, ui.KeyV)
	tt.Frame()
	golden.AssertMatrix(t, "shortcuts-recorded", p.View, paletteGoldens)
}
