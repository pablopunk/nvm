package app

import (
	"context"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/ai/quota"
	"github.com/pablopunk/nvm/native/internal/platform/folders"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/ai/extai"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/features/ocr"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
	"github.com/pablopunk/nvm/native/internal/store"
	"github.com/pablopunk/nvm/native/internal/windowmgmt"
)

// extensionHost holds the extension manager once it has started. The manager is built off the first-paint path
// and published atomically, so searches read nil until it exists and then the live manager.
type extensionHost struct {
	mgr atomic.Pointer[manager.Manager]
}

// extensionManager returns the running manager, or nil while extensions are still loading.
func (p *App) extensionManager() *manager.Manager {
	return p.exts.mgr.Load()
}

// startExtensions builds the manager from the extensions folder and publishes it. It runs on its own goroutine:
// scanning the folder and starting enabled children never waits the palette's first paint.
func (p *App) startExtensions() {
	m, err := p.newExtensionManager()
	if err != nil {
		log.Println("extensions:", err)
		return
	}
	p.exts.mgr.Store(m)
	log.Printf("extensions: %d installed", len(m.List()))
}

// newExtensionManager builds the manager over the app's one port set, so extensions see the same capabilities and
// system port as the palette does (the automation build swaps both in appPorts).
func (p *App) newExtensionManager() (*manager.Manager, error) {
	ports := p.ports
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	state, err := store.Open(ports.FS, ports.Clock, filepath.Join(dataDir(), "extension-state"))
	if err != nil {
		return nil, err
	}
	mygo.App.OnQuit(closeStoreOnQuit(state))
	textReader := ocr.New(ocr.Deps{Text: ports.Text, Capture: ports.Capture, FS: ports.FS, TempDir: os.TempDir()})
	return manager.New(manager.Ports{
		Files:     ports.FS,
		Clock:     ports.Clock,
		Worker:    real.NewWorker(),
		Processes: ports.Processes,
		Keychain:  ports.Keychain,
		Network:   ports.PublicHTTP,
		Clipboard: ports.Clipboard,
		Logs:      logx.Default(),

		Shell:         ports.Shell,
		Commands:      ports.Commands,
		Capabilities:  ports.Capabilities,
		OS:            p.goos,
		Windows:       p.extensionWindows(),
		Frontmost:     ports.Windows,
		Notifier:      ports.Notifier,
		Shortcuts:     p.extensionShortcuts(),
		OCR:           textReader,
		AI:            p.extensionAI(textReader),
		History:       p.extensionHistory,
		Confirm:       p.askExtensionConfirmation,
		Notify:        p.notifyExtensionFailure,
		SettingsPanes: p.systemSettingsPanes,
		Places:        p.systemPlaces,
		Indicators:    p.extensionIndicators(),
		Selection:     p.extensionSelection(),
	}, extensionsDir(), enablement.Open(state), manager.Options{
		Executable:      exe,
		DataRoot:        filepath.Join(dataDir(), "extension-data"),
		Home:            userHomeDir(),
		MenuBarRefresh:  p.refreshMenuBar,
		RootRefresh:     p.refreshRootRows,
		SearchRefresh:   func(_, query string) { p.RefreshSearch(query) },
		OnChange:        p.extensionsStateChanged,
		BuiltIns:        builtInExtensions(builtInInputs{Rates: p.rateCache, Zone: p.zone, Quicklinks: p.S.Quicklinks, GuideDone: p.S.GuideDone, SearchEngine: p.S.Engine}),
		AIQuota:         p.aiQuotas(),
		LegacyStatePath: p.legacyStatePath(),
	})
}

// aiQuotas is the app's model-call counts, one per owner: each extension, and the picker under its built-in id.
func (p *App) aiQuotas() *quota.Registry {
	if p.quotas == nil {
		p.quotas = quota.NewRegistry(p.ports.Clock, quota.CallsPerMinute)
	}
	return p.quotas
}

// extensionAI is the model call service extensions use. It shares the app's account and OCR, so a call made while
// signed out is refused the same way the palette's AI features refuse it.
func (p *App) extensionAI(textReader *ocr.Service) *extai.Service {
	return extai.New(extai.Deps{
		Client:    p.backendClient,
		Connected: p.signedIn,
		Clock:     p.ports.Clock,
		Clipboard: p.ports.Clipboard,
		Files:     p.ports.FS,
		OCR:       textReader,
		Home:      userHomeDir(),
	})
}

// extensionWindows is the window engine as extensions see it. The engine is attached before extensions start.
func (p *App) extensionWindows() services.WindowEngine {
	if p.windows == nil {
		return nil
	}
	return windowmgmt.NewExtension(p.windows)
}

// extensionShortcuts is the shortcut table as extensions read it; the manager exists before extensions start.
func (p *App) extensionShortcuts() services.ShortcutTable {
	return shortcuts.NewExtensionTable(p.shortcutsManager(), p.labeler)
}

// extensionHistory returns the clipboard history, or nil while the clipboard engine is not open.
func (p *App) extensionHistory() *clipboard.History {
	if p.clip == nil {
		return nil
	}
	return p.clip.history
}

// extensionsStateChanged redraws the views that show extension state when they are on top: the Extensions list, so a
// child that finishes starting shows as Running, and the root, so its rows appear without a new query. It also asks
// the menu bar to reconcile its items with the new state. It runs on the goroutine that changed the state.
func (p *App) extensionsStateChanged() {
	p.rootRows.reset()
	p.RefreshTop(isExtensionStateView)
	p.syncMenuBarItems()
}

// isExtensionStateView is true for the root and the Extensions list, the two views whose rows come from extension state.
func isExtensionStateView(v *palette.View) bool {
	return v.Title == extensionsViewTitle || v.Title == rootViewTitle
}

// refreshRootRows runs the root's rows again when the root is on top and shown, and asks only the extension that called.
// An extension's InvalidateRoot call arrives on the child's exchange goroutine, and RefreshTop is safe from any
// goroutine, so the call does not wait.
func (p *App) refreshRootRows(extensionID string) {
	p.rootRows.markPending(extensionID)
	p.RefreshTop(isRootView)
}

// isRootView is true for the palette's first level.
func isRootView(v *palette.View) bool {
	return v.Title == rootViewTitle
}

// questionPresenter is the part of the palette a host question needs: it shows the palette when it is hidden, puts
// the question on top, and takes the question away again.
type questionPresenter interface {
	Visible() bool
	Show()
	Hide()
	Push(v any)
	PopIf(v *palette.View) bool
}

// askExtensionConfirmation puts a destructive extension call's question on the palette and waits for the answer. It
// runs on the extension's goroutine; every palette change happens on the UI thread.
func (p *App) askExtensionConfirmation(ctx context.Context, question services.Question) (bool, error) {
	return awaitQuestion(ctx, p.onUI, p, question)
}

// awaitQuestion shows question above the palette's current page and returns the answer. A hidden palette is shown for
// the question and hidden again afterwards; a visible one keeps its page. A question whose context ends, such as a
// stopped extension, is taken off the palette and refused.
func awaitQuestion(ctx context.Context, onUI func(func()), pal questionPresenter, question services.Question) (bool, error) {
	answers := make(chan bool, 1)
	detail := fmt.Sprintf("Requested by %s. This cannot be undone.", question.Extension)
	var view *palette.View
	var wasVisible bool
	onUI(func() {
		wasVisible = pal.Visible()
		if !wasVisible {
			pal.Show()
		}
		view = palette.Decide(question.Action, detail, func(confirmed bool) { answers <- confirmed })
		pal.Push(view)
	})
	restore := func() {
		onUI(func() {
			if !wasVisible && pal.Visible() {
				pal.Hide()
			}
		})
	}
	select {
	case confirmed := <-answers:
		restore()
		return confirmed, nil
	case <-ctx.Done():
		onUI(func() { pal.PopIf(view) })
		restore()
		return false, ctx.Err()
	}
}

// userHomeDir is the folder a leading "~" in extension shell arguments stands for. Empty when the system has none.
func userHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// extensionsDir is the shared folder of extensions: one folder per Go extension, named after its id, beside the
// Electron app's TypeScript files. It is the folder the Electron app uses on every platform, so the two apps read
// the same extensions. docs/extension-api.md section 24 states the layout.
func extensionsDir() string {
	d := filepath.Join(sharedDir(), folders.ExtensionsName)
	_ = os.MkdirAll(d, 0o755)
	return d
}
