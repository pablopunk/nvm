package app

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// shownWindow is the palette's window in tests that hand focus back: showing and hiding only record the call.
type shownWindow struct{ visible bool }

func (w *shownWindow) Update(fn func())          { fn() }
func (w *shownWindow) Show()                     { w.visible = true }
func (w *shownWindow) Hide()                     { w.visible = false }
func (w *shownWindow) IsVisible() bool           { return w.visible }
func (w *shownWindow) IsFocused() bool           { return w.visible }
func (w *shownWindow) Size() (width, height int) { return 720, 470 }
func (w *shownWindow) SetPosition(x, y int)      {}

func windowCommandIDs(p *App) map[string]bool {
	ids := map[string]bool{}
	for _, command := range p.commands(false) {
		ids[command.ID] = true
	}
	return ids
}

func TestWindowCommandsAreListedWhereTheSystemManagesWindows(t *testing.T) {
	p := goldenPalette(t)
	p.attachWindows()
	if !windowCommandIDs(p)["window.left-half"] || !windowCommandIDs(p)["window.switch"] {
		t.Fatal("window commands are missing on a system that manages windows")
	}

	var withoutWindows []platform.Capability
	for _, capability := range platform.AllCapabilities() {
		if capability != platform.CapabilityWindowManagement {
			withoutWindows = append(withoutWindows, capability)
		}
	}
	p.ports.Capabilities = platform.NewCapabilities(withoutWindows...)
	if ids := windowCommandIDs(p); ids["window.left-half"] || ids["window.switch"] {
		t.Fatal("window commands are listed on a system without window management")
	}
}

func TestPaletteHandsFocusBackToTheWindowItWasSummonedFrom(t *testing.T) {
	p := goldenPalette(t)
	p.attachWindows()
	palette := &shownWindow{}
	p.Attach(palette)
	windows := p.ports.Windows.(*fake.WindowManager)
	summoned := windows.AddWindow(fake.SimWindow{PID: 42, BundleID: "app.notes", AppName: "Notes", Title: "Notes",
		Frame: platform.WindowFrame{X: 10, Y: 10, Width: 300, Height: 300}})
	other := windows.AddWindow(fake.SimWindow{PID: 77, BundleID: "app.other", AppName: "Other", Title: "Other",
		Frame: platform.WindowFrame{X: 400, Y: 10, Width: 300, Height: 300}})
	windows.Focus(summoned)
	focus := p.ports.Focus.(*fake.Focus)
	focus.SetActive(platform.FocusTarget{Window: "42-window", PID: 42})
	p.Show()

	target, ok := p.windowTarget()
	if !ok || target.PID != 42 {
		t.Fatalf("command target = %+v (known %v), want the notes app", target, ok)
	}
	windows.Focus(other)
	if !p.HidePaletteAndRestoreFocus(context.Background(), target) {
		t.Fatal("focus was not handed back")
	}
	if palette.IsVisible() {
		t.Error("the palette is still visible after the hand-back")
	}
	restored := focus.Restored()
	if len(restored) == 0 {
		t.Fatal("nothing was restored")
	}
	for _, target := range restored {
		if target.Window != "42-window" {
			t.Errorf("restored %+v, want only the summoned window", target)
		}
	}
}

func TestSummonCapturesNothingOfItsOwnWhereThePaletteHasAFocusPort(t *testing.T) {
	p := goldenPalette(t)
	p.attachWindows()
	p.Attach(&shownWindow{})
	p.ports.Windows.(*fake.WindowManager).AddWindow(fake.SimWindow{PID: 9, BundleID: "app.x", AppName: "X", Title: "X",
		Frame: platform.WindowFrame{Width: 100, Height: 100}})
	p.onPaletteShown()
	if p.windowFocus.load().known {
		t.Fatal("the window commands captured a second target on a palette summon")
	}
}

func TestShortcutsListWindowCommandsAndBindThemLikeAnyAction(t *testing.T) {
	p := goldenPalette(t)
	p.attachWindows()
	if err := p.shortcutsManager().SetAction("window.left-half", "Control+Alt+Left"); err != nil {
		t.Fatal(err)
	}
	for _, entry := range p.shortcutsManager().Entries() {
		if entry.ActionID == "window.left-half" {
			if entry.Title != "Left Half" || entry.Accelerator != "Control+Alt+Left" {
				t.Fatalf("entry = %+v, want Left Half on Control+Alt+Left", entry)
			}
			return
		}
	}
	t.Fatal("the Keyboard Shortcuts list has no Left Half row")
}
