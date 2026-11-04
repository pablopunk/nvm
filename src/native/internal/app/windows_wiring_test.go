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
	palette := &shownWindow{visible: true}
	p.Attach(palette)
	windows := p.ports.Windows.(*fake.WindowManager)
	summoned := windows.AddWindow(fake.SimWindow{PID: 42, BundleID: "app.notes", AppName: "Notes", Title: "Notes",
		Frame: platform.WindowFrame{X: 10, Y: 10, Width: 300, Height: 300}})
	other := windows.AddWindow(fake.SimWindow{PID: 77, BundleID: "app.other", AppName: "Other", Title: "Other",
		Frame: platform.WindowFrame{X: 400, Y: 10, Width: 300, Height: 300}})
	windows.Focus(summoned)
	p.captureWindowFocus()

	target, known := p.PaletteFocusTarget()
	if !known || target.PID != 42 {
		t.Fatalf("captured target = %+v (known %v), want the notes app", target, known)
	}
	windows.Focus(other)
	if !p.HidePaletteAndRestoreFocus(context.Background(), target) {
		t.Fatal("focus was not handed back")
	}
	if palette.IsVisible() {
		t.Error("the palette is still visible after the hand-back")
	}
	front, found, _ := windows.Frontmost(context.Background())
	if !found || front.PID != target.PID {
		t.Errorf("front app = %+v, want the notes app", front)
	}
}
