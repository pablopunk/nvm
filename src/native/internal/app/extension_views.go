package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/palette/feedback"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/search"
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
		OpenURL:               func(url string) { _ = mygo.Shell.OpenExternal(url) },
		CopyText:              func(h search.Host, text string) { p.copyText(h, text) },
		Paste:                 func(h search.Host, extID string, action api.Action) { p.pasteFromExtension(h, m, extID, action) },
		OpenSettingsPane:      p.openSystemPaneByID,
		RecordPaletteShortcut: func(h search.Host) { h.Push(p.paletteRecorderView()) },
		ReadSetting:           func(extID, key string) (json.RawMessage, error) { return m.SettingValue(extID, key) },
		RowAccessory:          p.settingRowAccessory,
		HostOperation:         p.settingsHostOperation,
		RecordSetting:         p.settingRecorder,
		WriteSetting: func(extID, key string, value json.RawMessage) error {
			return m.WriteSetting(extID, key, value)
		},
		SignIn:         p.signInFromRoot,
		OpenPath:       func(path string) { _ = ports.Shell.OpenPath(path) },
		RevealPath:     func(path string) { _ = ports.Shell.Reveal(path) },
		RunSystem:      p.runSystemOp,
		RunUpdate:      p.runUpdateOp,
		ShortcutAction: p.runShortcutAction,
		HyperKey:       p.hyperKey,
		OnUI:           p.onUI,
		Files:          ports.FS,
		Clock:          ports.Clock,
		Cache:          extensionSnapshots,
		Dialog:         p.GuardDialog(ports.FileDialog),
		Indicate:       p.extensionIndicators().ShowIndicator,
		Drafts:         m,
		Pages:          real.NewWebPages(p.paletteWebWindow),
		Permissions:    ports.Permissions,
		Cameras:        extensionCameras,
		IsDark:         func() bool { return mygo.Theme.IsDark() },
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

// pasteFromExtension hides the palette, then has the extension's services send the paste or typing action to the app
// in front, off the UI thread. The HUD says Pasted or Typed, or the reason the action could not run.
func (p *App) pasteFromExtension(h search.Host, m *manager.Manager, extID string, action api.Action) {
	title := action.Title
	if title == "" {
		title = pasteActionTitle
	}
	done := "Pasted"
	if action.Kind == api.ActionTypeText {
		done = "Typed"
	}
	p.hideThenRun(h, title, done, func() error { return m.PerformPaste(extID, action) }, func(err error) palette.Feedback {
		return palette.Feedback{Title: title, Message: err.Error(), Tone: feedback.ToneError}
	})
}
