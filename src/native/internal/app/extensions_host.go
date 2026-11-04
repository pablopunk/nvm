package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"

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
	state, err := store.Open(ports.FS, ports.Clock, filepath.Join(configDir(), "extension-state"))
	if err != nil {
		return nil, err
	}
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
		Capabilities:  ports.Capabilities,
		OS:            runtime.GOOS,
		Windows:       p.extensionWindows(),
		Shortcuts:     p.extensionShortcuts(),
		OCR:           ocr.New(ocr.Deps{Text: ports.Text, Capture: ports.Capture, FS: ports.FS, TempDir: os.TempDir()}),
		History:       p.extensionHistory,
		Confirm:       p.askExtensionConfirmation,
		SettingsPanes: p.systemSettingsPanes,
	}, extensionsDir(), enablement.Open(state), manager.Options{
		Executable:     exe,
		DataRoot:       filepath.Join(configDir(), "extension-data"),
		MenuBarRefresh: p.refreshMenuBar,
		OnChange:       p.extensionsStateChanged,
		BuiltIns:       builtInExtensions(),
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
	return shortcuts.NewExtensionTable(p.shortcutsManager())
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
	p.RefreshTop(isExtensionStateView)
	p.syncMenuBarItems()
}

// isExtensionStateView is true for the root and the Extensions list, the two views whose rows come from extension state.
func isExtensionStateView(v *palette.View) bool {
	return v.Title == extensionsViewTitle || v.Title == rootViewTitle
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

// extensionsDir is the folder of installed extensions: one folder per extension, named after its id.
func extensionsDir() string {
	d := filepath.Join(configDir(), "extensions")
	_ = os.MkdirAll(d, 0o755)
	return d
}
