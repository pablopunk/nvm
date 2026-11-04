package app

import (
	"log"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/store"
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
	m, err := newExtensionManager()
	if err != nil {
		log.Println("extensions:", err)
		return
	}
	p.exts.mgr.Store(m)
	log.Printf("extensions: %d installed", len(m.List()))
}

func newExtensionManager() (*manager.Manager, error) {
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
	}, extensionsDir(), enablement.Open(state), manager.Options{
		Executable: exe,
		DataRoot:   filepath.Join(configDir(), "extension-data"),
	})
}

// extensionsDir is the folder of installed extensions: one folder per extension, named after its id.
func extensionsDir() string {
	d := filepath.Join(configDir(), "extensions")
	_ = os.MkdirAll(d, 0o755)
	return d
}
