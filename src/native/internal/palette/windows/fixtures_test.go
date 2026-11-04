package windows

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/windowmgmt"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

const (
	targetPID = 42
	otherPID  = 77
	selfPID   = 9
	// watchdog bounds a wait for a background notification. It never stands in for one.
	watchdog = 5 * time.Second
)

// recordingHost is the palette host the rows run against. It records what the rows asked the palette to do.
type recordingHost struct {
	pushed  []any
	toasts  []string
	pops    int
	refresh int
}

func (h *recordingHost) Hide()      {}
func (h *recordingHost) Pop()       { h.pops++ }
func (h *recordingHost) Push(v any) { h.pushed = append(h.pushed, v) }
func (h *recordingHost) Toast(msg string) {
	h.toasts = append(h.toasts, msg)
}
func (h *recordingHost) Refresh()          { h.refresh++ }
func (h *recordingHost) Query() string     { return "" }
func (h *recordingHost) SetQuery(q string) {}

// inLine runs background work on the calling goroutine, so a test sees its effects when the call returns.
func inLine(_ string, fn func(context.Context)) {
	fn(context.Background())
}

// desk is a simulated desktop with a 1000 by 800 display, a target app with a focused window, and a settings
// store in memory. Notifications are collected instead of shown.
type desk struct {
	windows  *fake.WindowManager
	deps     Deps
	settings *windowmgmt.SettingsStore
	notes    chan [2]string
	target   windowmgmt.Target
	focus    *paletteFocus
	key      string
}

// paletteFocus stands in for the palette's focus hand-back: it hides nothing and focuses the target's window.
type paletteFocus struct {
	windows *fake.WindowManager
	target  windowmgmt.Target
	key     string
}

func (f *paletteFocus) PaletteFocusTarget() (windowmgmt.Target, bool) {
	return f.target, true
}

func (f *paletteFocus) HidePaletteAndRestoreFocus(context.Context, windowmgmt.Target) bool {
	f.windows.Focus(f.key)
	return true
}

func newDesk(t *testing.T) desk {
	t.Helper()
	return buildDesk(t, true)
}

// newEmptyDesk is a desktop with a display and no windows.
func newEmptyDesk(t *testing.T) desk {
	t.Helper()
	return buildDesk(t, false)
}

func buildDesk(t *testing.T, withWindows bool) desk {
	t.Helper()
	windows := fake.NewWindowManager()
	windows.SetDisplays(platform.WindowDisplay{
		ID: "main", Primary: true,
		Frame:    platform.WindowFrame{Width: 1000, Height: 800},
		WorkArea: platform.WindowFrame{Width: 1000, Height: 800},
	})
	targetKey := ""
	if withWindows {
		targetKey = windows.AddWindow(fake.SimWindow{PID: targetPID, BundleID: "app.target", AppName: "Notes",
			Title: "Meeting notes", Frame: platform.WindowFrame{X: 100, Y: 100, Width: 300, Height: 300}})
		windows.AddWindow(fake.SimWindow{PID: otherPID, BundleID: "app.other", AppName: "Terminal",
			Title: "zsh", Frame: platform.WindowFrame{X: 500, Y: 100, Width: 300, Height: 300}})
		windows.Focus(targetKey)
	}
	target := windowmgmt.Target{PID: targetPID, BundleID: "app.target"}
	focus := &paletteFocus{windows: windows, target: target, key: targetKey}
	engine := windowmgmt.NewService(windowmgmt.Deps{Windows: windows, Focus: focus, Clock: fake.NewClock(), SelfPID: selfPID})
	settings := windowmgmt.NewSettingsStore(nil)
	d := desk{windows: windows, settings: settings, notes: make(chan [2]string, 8), target: target, focus: focus, key: targetKey}
	d.deps = Deps{
		Engine:   engine,
		Settings: settings,
		Target:   func() (windowmgmt.Target, bool) { return d.target, true },
		OnUI:     func(fn func()) { fn() },
		Notify:   func(title, body string) { d.notes <- [2]string{title, body} },
		Go:       inLine,
	}
	return d
}

// awaitNote returns the next notification, failing the test after the watchdog.
func (d desk) awaitNote(t *testing.T) [2]string {
	t.Helper()
	select {
	case note := <-d.notes:
		return note
	case <-time.After(watchdog):
		t.Fatal("no notification")
	}
	return [2]string{}
}

// goldenPalette is a palette on the root with the given view above it, as the goldens render it.
func goldenPalette(t *testing.T, view *palette.View) *palette.Palette {
	t.Helper()
	root := &palette.View{Title: "Nevermind", Search: func(string) []search.Item { return nil }}
	p := palette.New(palette.Options{Root: root, Clock: fake.NewClock(), Synchronous: true})
	p.SetStack(root, view)
	p.SetQuery("")
	return p
}

// windowmgmtSettingsWith is a settings store in memory that starts from the defaults with change applied.
func windowmgmtSettingsWith(t *testing.T, change func(*geometry.Settings)) *windowmgmt.SettingsStore {
	t.Helper()
	settings := windowmgmt.NewSettingsStore(nil)
	if err := settings.Update(change); err != nil {
		t.Fatal(err)
	}
	return settings
}

// Root has nothing to leave in this test host.
func (h *recordingHost) Root() {}
