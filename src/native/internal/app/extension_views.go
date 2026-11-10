package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/features/files/thumbs"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/palette/feedback"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/search"
)

// extensionRunTimeout bounds one action an extension runs from the palette. Actions run off the UI thread.
const extensionRunTimeout = 10 * time.Second

// Refreshes run behind a visible snapshot and may need to validate many local files.
const extensionRefreshTimeout = 2 * time.Minute

// backgroundRunTimeout bounds an action declared background, as the manager's background run budget does, plus slack.
const backgroundRunTimeout = 2*time.Minute + 10*time.Second

// settleExtensionCalls waits for the extension calls the palette started to return and to deliver their results. The
// app never waits for them before it draws; goldens and tests do, so they see the view the call opened.
func (p *App) settleExtensionCalls() {
	p.extCalls.Wait()
}

// extensionViewDeps are the services of extension views: an action runs on the manager's supervised child, and its
// result returns to the UI thread, where the palette applies it.
func (p *App) extensionViewDeps(m *manager.Manager) extview.Deps {
	ports := p.ports
	invoke := func(base context.Context, command bool, extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
		p.beginPaletteRun()
		p.extCalls.Add(1)
		go func() {
			defer p.extCalls.Done()
			ctx, cancel := context.WithTimeout(base, runTimeoutOf(in))
			defer cancel()
			run := m.Run
			if command {
				run = m.RunCommand
			}
			result, failure, err := run(ctx, extID, in)
			p.endPaletteRun()
			if err != nil {
				failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
			}
			p.onUI(func() { done(result, failure) })
		}()
	}
	return extview.Deps{
		CommandSnapshots:        &p.commandSnapshots,
		CommandSnapshotIdentity: m.SnapshotIdentity,
		InvokeRefresh: func(ctx context.Context, extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
			invoke(ctx, false, extID, in, done)
		},
		Invoke: func(extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
			invoke(context.Background(), false, extID, in, done)
		},
		InvokeCommand: func(extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
			invoke(manager.WithCommandRunOrigin(context.Background(), m.ResolveCommandOrigin(extID, in.Ref.ID)), true, extID, in, done)
		},
		InvokeCommandContext: func(ctx context.Context, extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
			invoke(manager.WithCommandRunOrigin(ctx, m.ResolveCommandOrigin(extID, in.Ref.ID)), true, extID, in, done)
		},
		InvokeViewAction: func(extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
			invoke(manager.WithViewActionRunOrigin(context.Background(), m.ResolveViewActionOrigin(extID)), false, extID, in, done)
		},
		Load: func(extID, loader string, done func([]api.Item, *api.SnapshotEvidence, *api.Failure)) {
			p.extCalls.Add(1)
			go func() {
				defer p.extCalls.Done()
				ctx, cancel := context.WithTimeout(context.Background(), extensionRunTimeout)
				defer cancel()
				items, evidence, failure, err := m.Load(ctx, extID, loader)
				if err != nil {
					failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
				}
				p.onUI(func() { done(items, evidence, failure) })
			}()
		},
		Answer: func(ctx context.Context, extID string, in api.Invocation) (api.Result, *api.Failure) {
			result, failure, err := m.Run(ctx, extID, in)
			if err != nil {
				failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
			}
			return result, failure
		},
		LoadGridImage: func(ctx context.Context, request extview.GridImageRequest, done func(extview.GridImageResult)) {
			p.loadScopedGridImage(ctx, m, request, done)
		},
		LoadImage: func(ctx context.Context, ref string, done func(*ui.Bitmap)) {
			go func() {
				var bitmap *ui.Bitmap
				if !thumbs.IsGridReference(ref) {
					bitmap = loadExtensionBitmapContext(ctx, ports.FS, ref, ports.PublicHTTP)
				}
				p.onUI(func() { done(bitmap) })
			}()
		},
		OpenURL:               func(url string) { _ = mygo.Shell.OpenExternal(url) },
		CopyText:              func(h search.Host, text string) { p.copyText(h, text) },
		CopyImage:             func(h search.Host, extID, path string, identity any) { p.copyImageFile(h, m, extID, path, identity) },
		LinkCard:              p.linkCardOf,
		Paste:                 func(h search.Host, extID string, action api.Action) { p.pasteFromExtension(h, m, extID, action) },
		RecordPaletteShortcut: func(h search.Host) { h.Push(p.paletteRecorderView()) },
		Window:                p.performExtensionWindow,
		ToggleSetting: func(extID, key string, identity any, done func(error)) {
			p.extCalls.Add(1)
			go func() {
				defer p.extCalls.Done()
				err := m.ToggleSettingAction(extID, key, identity)
				p.onUI(func() { done(err) })
			}()
		},
		ReadSetting:   func(extID, key string) (json.RawMessage, error) { return m.SettingValue(extID, key) },
		RowAccessory:  p.settingRowAccessory,
		ReadDisplay:   m.SharedDisplay,
		HostOperation: p.settingsHostOperation,
		RecordSetting: p.settingRecorder,
		WriteSetting: func(extID, key string, value json.RawMessage) error {
			return m.WriteSetting(extID, key, value)
		},
		RunUpdate:           p.runUpdateOp,
		ShortcutAction:      p.runShortcutAction,
		SignIn:              p.signInFromRoot,
		OpenPath:            func(path string) { _ = ports.Shell.OpenPath(path) },
		RevealPath:          func(path string) { _ = ports.Shell.Reveal(path) },
		RevealTitle:         platform.ShellLabelsFor(p.goos).RevealInFileManager,
		ActionSupported:     m.ActionSupported,
		OperationsSupported: m.OperationsSupported,
		PreviewFile:         p.extensionFilePreview(m),
		OpenWith:            func(path, appPath string) error { return ports.Shell.OpenWith(path, appDescriptor(appPath)) },
		Trash:               func(path string) error { return ports.Shell.Trash(path) },
		RunSystem:           p.runSystemOp,
		SystemCommand:       p.extensionSystemCommand(m),
		ShellAction:         p.extensionShellAction(m),
		HyperKey:            p.hyperKey,
		OnUI:                p.onUI,
		Files:               ports.FS,
		Clock:               ports.Clock,
		Cache:               extensionSnapshots,
		Dialog:              p.GuardDialog(ports.FileDialog),
		Indicate:            p.extensionIndicators().ShowIndicator,
		Drafts:              m,
		Pages:               real.NewWebPages(p.paletteWebWindow),
		Permissions:         ports.Permissions,
		Cameras:             extensionCameras,
		IsDark:              func() bool { return mygo.Theme.IsDark() },
		Reply:               p.drawReply,
		PickImages:          p.chooseChatImages,
		Chats:               extensionChatPages{p: p},
		Review:              p.builderReview,
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

// runTimeoutOf is the palette's bound on one action: a background action may wait for the microphone or a transcript.
func runTimeoutOf(in api.Invocation) time.Duration {
	if in.Background {
		return backgroundRunTimeout
	}
	if in.AwaitInitialView {
		return 2 * time.Minute
	}
	if in.Launch.Refresh {
		return extensionRefreshTimeout
	}
	return extensionRunTimeout
}
