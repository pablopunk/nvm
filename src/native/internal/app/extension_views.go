package app

import (
	"context"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// extensionRunTimeout bounds one action an extension runs from the palette. Actions run off the UI thread.
const extensionRunTimeout = 10 * time.Second

// extensionViewDeps are the services of extension views: an action runs on the manager's supervised child, and its
// result returns to the UI thread, where the palette applies it.
func (p *App) extensionViewDeps(m *manager.Manager) extview.Deps {
	ports := p.ports
	return extview.Deps{
		Invoke: func(extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), extensionRunTimeout)
				defer cancel()
				result, failure, err := m.Run(ctx, extID, in)
				if err != nil {
					failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
				}
				p.onUI(func() { done(result, failure) })
			}()
		},
		Load: func(extID, loader string, done func([]api.Item, *api.Failure)) {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), extensionRunTimeout)
				defer cancel()
				items, failure, err := m.Load(ctx, extID, loader)
				if err != nil {
					failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
				}
				p.onUI(func() { done(items, failure) })
			}()
		},
		LoadImage: func(ref string, done func(*ui.Bitmap)) {
			go func() {
				bitmap := loadExtensionBitmap(ports.FS, ref)
				p.onUI(func() { done(bitmap) })
			}()
		},
		OpenURL:          func(url string) { _ = mygo.Shell.OpenExternal(url) },
		CopyText:         func(text string) { mygo.Clipboard.WriteText(text) },
		OpenSettingsPane: p.openSystemPaneByID,
		OpenPath:         func(path string) { _ = ports.Shell.OpenPath(path) },
		RevealPath:       func(path string) { _ = ports.Shell.Reveal(path) },
		RunSystem:        p.runSystemOp,
		HyperKey:         p.hyperKey,
		Files:            ports.FS,
		Clock:            ports.Clock,
		Cache:            extensionSnapshots,
		Dialog:           p.GuardDialog(ports.FileDialog),
		Drafts:           m,
		Pages:            real.NewWebPages(p.paletteWebWindow),
		Permissions:      ports.Permissions,
		Cameras:          extensionCameras,
		IsDark:           func() bool { return mygo.Theme.IsDark() },
	}
}

// paletteWebWindow is the palette's window, which web pages of views go in. It is nil until the window exists, so
// a page opened before then is refused rather than placed nowhere.
func (p *App) paletteWebWindow() real.WebWindow {
	if p.panel == nil {
		return nil
	}
	return p.panel
}

// extensionSnapshots keeps data-source snapshots for the life of the process, so reopening a view shows its items at once.
var extensionSnapshots = extview.NewMemoryCache()

// extensionCameras keeps the camera each camera view chose for the life of the process, as the snapshots do.
var extensionCameras = extview.NewMemoryCameras()
