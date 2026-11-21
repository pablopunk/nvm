package app

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/windowmgmt"
)

// withoutFocusPort makes the palette look as it does on macOS: no focus port, so a summon captures only the app in front.
func withoutFocusPort(p *App) {
	var capabilities []platform.Capability
	for _, capability := range platform.AllCapabilities() {
		if capability != platform.CapabilityFocusReturn {
			capabilities = append(capabilities, capability)
		}
	}
	p.ports.Capabilities = platform.NewCapabilities(capabilities...)
}

// runPaletteCommand runs the left-half layout the way the built-in does for a palette run: the palette captures its
// window, hides, and the engine call follows. It returns the message the user would see, empty when the window reached
// its frame.
func runPaletteCommand(t *testing.T, p *App) string {
	t.Helper()
	p.beginPaletteRun()
	defer p.endPaletteRun()
	p.Palette.Hide()
	left := api.WindowLayout{Kind: api.LayoutFraction, Fraction: &api.Rect{Width: 0.5, Height: 1}, CycleGroup: "left"}
	result, err := p.extensionWindows().Apply(context.Background(), left, api.WindowManagementOptions{})
	switch {
	case err != nil:
		return err.Error()
	case !result.OK:
		return result.Message
	}
	return ""
}

func TestHandBackOnMacOSIsRequestedWithoutACapturedWindow(t *testing.T) {
	p := goldenPalette(t)
	p.attachWindows()
	withoutFocusPort(p)
	palette := &shownWindow{visible: true}
	p.Attach(palette)
	p.windowFocus.store(summonRecord{target: windowmgmt.Target{PID: 42, BundleID: "app.notes"}, known: true})

	if !p.HidePaletteAndRestoreFocus(context.Background(), windowmgmt.Target{PID: 42}) {
		t.Fatal("the hand-off was refused although the palette has no focus port to restore")
	}
	if palette.IsVisible() {
		t.Error("the palette is still visible after the hand-off")
	}
}

func TestPaletteWindowCommandMovesTheAppThatWasFrontWhenSummoned(t *testing.T) {
	p := goldenPalette(t)
	p.attachWindows()
	withoutFocusPort(p)
	palette := &shownWindow{}
	p.Attach(palette)
	windows := p.ports.Windows.(*fake.WindowManager)
	windows.SetDisplays(platform.WindowDisplay{ID: "main", Primary: true,
		Frame:    platform.WindowFrame{Width: 1000, Height: 800},
		WorkArea: platform.WindowFrame{Y: 25, Width: 1000, Height: 775}})
	notes := windows.AddWindow(fake.SimWindow{PID: 42, BundleID: "app.notes", AppName: "Notes", Title: "Notes",
		Frame: platform.WindowFrame{X: 400, Y: 100, Width: 300, Height: 300}})
	other := windows.AddWindow(fake.SimWindow{PID: 77, BundleID: "app.other", AppName: "Other", Title: "Other",
		Frame: platform.WindowFrame{X: 400, Y: 400, Width: 300, Height: 300}})
	windows.Focus(notes)
	windows.Focus(other)
	windows.Focus(notes)
	p.Show()

	if message := runPaletteCommand(t, p); message != "" {
		t.Fatalf("left half refused: %s", message)
	}
	if palette.IsVisible() {
		t.Error("the palette is still visible after the command")
	}
	moved, _ := windows.Window(notes)
	if moved.Frame.X > 16 || moved.Frame.X+moved.Frame.Width > 500 {
		t.Errorf("notes frame = %+v, want the left half of the display", moved.Frame)
	}
	untouched, _ := windows.Window(other)
	if untouched.Frame.X != 400 {
		t.Errorf("the other app's window moved to %+v", untouched.Frame)
	}
}

func TestPaletteWindowCommandSaysSoWhenNoAppWasCaptured(t *testing.T) {
	p := goldenPalette(t)
	p.attachWindows()
	withoutFocusPort(p)
	p.Attach(&shownWindow{})
	windows := p.ports.Windows.(*fake.WindowManager)
	windows.SetDisplays(platform.WindowDisplay{ID: "main", Primary: true, WorkArea: platform.WindowFrame{Width: 1000, Height: 800}})
	notes := windows.AddWindow(fake.SimWindow{PID: 42, BundleID: "app.notes", AppName: "Notes", Title: "Notes",
		Frame: platform.WindowFrame{X: 200, Width: 300, Height: 300}})
	windows.Focus(notes)
	windows.FailNext("Frontmost", platform.ErrNotFound)
	p.Show()

	if message := runPaletteCommand(t, p); message == "" {
		t.Fatal("the command reported success without an app to act on")
	}
	if unmoved, _ := windows.Window(notes); unmoved.Frame.X != 200 {
		t.Errorf("a window moved although no app was captured: %+v", unmoved.Frame)
	}
}
