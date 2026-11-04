package app

import (
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

func (p *App) newExtensionManager() (*manager.Manager, error) {
	ports := real.NewPorts()
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

		Shell:        ports.Shell,
		Capabilities: ports.Capabilities,
		OS:           runtime.GOOS,
		Windows:      p.extensionWindows(),
		Shortcuts:    p.extensionShortcuts(),
		OCR:          ocr.New(ocr.Deps{Text: ports.Text, Capture: ports.Capture, FS: ports.FS, TempDir: os.TempDir()}),
		History:      p.extensionHistory,
	}, extensionsDir(), enablement.Open(state), manager.Options{
		Executable: exe,
		DataRoot:   filepath.Join(configDir(), "extension-data"),
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

// extensionsDir is the folder of installed extensions: one folder per extension, named after its id.
func extensionsDir() string {
	d := filepath.Join(configDir(), "extensions")
	_ = os.MkdirAll(d, 0o755)
	return d
}
