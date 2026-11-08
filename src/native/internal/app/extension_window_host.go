package app

import (
	"context"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/palette/extwindow"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/store"
)

// extensionWindowsFile is the document where the persistent extension windows and their frames are kept.
const extensionWindowsFile = "extension-windows.json"

// extensionWindowRestoreTimeout bounds the call that rebuilds one persistent window after a relaunch.
const extensionWindowRestoreTimeout = 10 * time.Second

// attachExtensionWindows builds the manager of the windows extensions open. Building it opens no window.
func (p *App) attachExtensionWindows() {
	p.extWindows = extwindow.New(extwindow.Deps{
		Open:      extWindowOpener{p: p},
		Page:      p.extensionWindowPage,
		PageBound: p.extensionWindowPageBound,
		Saved:     p.extensionWindowState(),
		Clock:     p.ports.Clock,
		OnUI:      p.onUI,
	})
}

// extensionWindowState is the store of the persistent windows and frames, or nil when the app has no store.
func (p *App) extensionWindowState() extwindow.Saver {
	if p.store == nil {
		return nil
	}
	return store.Doc(p.store, extensionWindowsFile, 1, func() extwindow.Saved { return extwindow.Saved{} })
}

// extensionWindowPage draws an extension's view for a window, with the window's palette as the host of its loads.
// It is nil while the extension host is not running.
func (p *App) extensionWindowPage(extID string, view api.View, host search.Host) *palette.View {
	m := p.extensionManager()
	if m == nil {
		return nil
	}
	return extview.NewSession(p.extensionViewDeps(m), extID, view).Open(host)
}

// extensionWindowPageBound keeps child actions on the installation that produced the view.
func (p *App) extensionWindowPageBound(extID string, identity any, view api.View, host search.Host) *palette.View {
	m := p.extensionManager()
	if m == nil {
		return nil
	}
	return extview.NewSessionForIdentity(p.extensionViewDeps(m), extID, view, identity).Open(host)
}

// performExtensionWindow runs a window action an extension's view asked for. The palette has hidden already, so a
// failure is shown as a toast on the app.
func (p *App) performExtensionWindow(extID string, action api.Action) {
	if err := p.extWindows.Perform(extID, action); err != nil {
		p.Toast(err.Error())
	}
}

// performExtensionWindowBound admits the captured installation before any window effect.
func (p *App) performExtensionWindowBound(m *manager.Manager, extID string, identity any, action api.Action) {
	authority, err := m.WindowAuthorityFor(extID, identity)
	if err == nil {
		err = p.extWindows.PerformBound(extID, authority, action)
	}
	if err != nil {
		p.Toast(err.Error())
	}
}

// restoreExtensionWindows reopens the persistent windows once the extension manager runs, asking each owning
// extension for its fresh view.
func (p *App) restoreExtensionWindows(m *manager.Manager) {
	p.extWindows.RestorePersistedBound(func(extID, restoreKey string) (*api.WindowRestore, extwindow.Authority, error) {
		ctx, cancel := context.WithTimeout(context.Background(), extensionWindowRestoreTimeout)
		defer cancel()
		return m.RestoreWindowBound(ctx, extID, restoreKey)
	})
}

// extWindowOpener opens the native windows: a palette that draws the window's page, in a MyGo window that starts
// hidden.
type extWindowOpener struct{ p *App }

func (o extWindowOpener) Open(spec extwindow.Spec) extwindow.Surface {
	pal := palette.New(palette.Options{
		Clock:   o.p.ports.Clock,
		Icon:    o.p.fileIcon,
		Learner: o.p.learn,
		DragRow: o.p.dragRow,
		Layout:  extensionWindowLayout(spec),
	})
	frame := o.p.extensionWindowFrame(spec)
	options := mygo.WindowOptions{
		Title: spec.Title, Width: frame.Width, Height: frame.Height, X: frame.X, Y: frame.Y,
		MinWidth: 320, MinHeight: 240, AlwaysOnTop: spec.AlwaysOnTop, Hidden: true, Frameless: spec.Frameless,
		Content: ui.View(pal.View),
	}
	if spec.TitleBarHidden {
		options.TitleBarStyle = mygo.TitleBarHiddenInset
		options.TrafficLightPosition = &mygo.Point{X: 12, Y: 12}
	}
	window := mygo.NewWindow(options)
	pal.Attach(window)
	surface := extWindowSurface{pal: pal, window: window}
	surface.Reconfigure(spec)
	return surface
}

// extensionWindowLayout is how a window's palette sits in it: a companion when the title bar is hidden, otherwise the
// window layout with the native title bar.
func extensionWindowLayout(spec extwindow.Spec) palette.WindowLayout {
	if spec.TitleBarHidden {
		return palette.LayoutCompanion
	}
	return palette.LayoutWindow
}

// extensionWindowFrame is where a window opens: the frame it remembers, moved onto a connected display, or a place on
// the display under the pointer.
func (p *App) extensionWindowFrame(spec extwindow.Spec) extwindow.Frame {
	if spec.Saved != nil {
		return extwindow.FitFrame(*spec.Saved, p.displays())
	}
	return extwindow.PlaceFrame(p.cursorWorkArea(), spec.Width, spec.Height, spec.Position)
}

// extWindowSurface is an open extension window: its palette shows the page, and the MyGo window holds the palette.
type extWindowSurface struct {
	pal    *palette.Palette
	window *mygo.Window
}

func (s extWindowSurface) Show()                      { s.pal.Show() }
func (s extWindowSurface) Hide()                      { s.pal.Hide() }
func (s extWindowSurface) Close()                     { s.window.Close() }
func (s extWindowSurface) IsVisible() bool            { return s.window.IsVisible() }
func (s extWindowSurface) Host() search.Host          { return s.pal }
func (s extWindowSurface) OnBlur(fn func())           { s.window.OnBlur(fn) }
func (s extWindowSurface) OnClosed(fn func())         { s.window.OnClosed(fn) }
func (s extWindowSurface) OnFrameChanged(fn func())   { s.window.OnMove(fn); s.window.OnResize(fn) }
func (s extWindowSurface) SetRoot(root *palette.View) { s.pal.SetStack(root); s.pal.Refresh() }

func (s extWindowSurface) Reconfigure(spec extwindow.Spec) {
	s.window.SetTitle(spec.Title)
	s.window.SetSize(spec.Width, spec.Height)
	s.window.SetAlwaysOnTop(spec.AlwaysOnTop)
	s.window.SetVisibleOnAllWorkspaces(spec.VisibleOnAllSpaces)
}

func (s extWindowSurface) Frame() extwindow.Frame {
	x, y := s.window.Position()
	width, height := s.window.Size()
	return extwindow.Frame{X: x, Y: y, Width: width, Height: height}
}
