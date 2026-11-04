package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	clipboardbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/clipboard"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/store"
)

// clipboardBuiltInView opens the Clipboard History built-in's list over the engine's history, as the palette opens it.
func clipboardBuiltInView(t testing.TB, engine *clipEngine) *palette.View {
	t.Helper()
	m := newManagerOverClipboard(t, engine)
	return builtInViewPage(t, m, clipboardbuiltin.ID, api.ActionRef{ID: clipboardbuiltin.ListCommandID})
}

// newManagerOverClipboard is a manager with the built-ins, whose clipboard history and pictures are the engine's. The
// clipboard port stays empty: the list and the removals read the history only.
func newManagerOverClipboard(t testing.TB, engine *clipEngine) *manager.Manager {
	t.Helper()
	builtins := builtInExtensions(builtInInputs{Zone: time.UTC, Clipboard: defaultClipboardOptions})
	dir := t.TempDir()
	ports := real.NewPorts()
	state, err := store.Open(ports.FS, ports.Clock, filepath.Join(dir, "state"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = state.Close() })
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	m, err := manager.New(manager.Ports{
		Files: ports.FS, Clock: ports.Clock, Worker: real.NewWorker(), Processes: ports.Processes,
		Capabilities: platform.NewCapabilities(platform.AllCapabilities()...),
		Places:       fixturePlaces,
		History:      func() *clipboard.History { return engine.history },
		ClipboardImages: services.ClipboardImages(engine.files),
	}, filepath.Join(dir, "extensions"), enablement.Open(state), manager.Options{
		Executable: exe,
		DataRoot:   filepath.Join(dir, "extension-data"),
		BuiltIns:   builtins,
		Budgets: map[string]supervisor.Budget{
			protocol.MethodSearch: {Soft: 50 * time.Millisecond, Hard: 300 * time.Millisecond},
		},
	})
	if err != nil {
		t.Fatalf("manager.New: %v", err)
	}
	t.Cleanup(m.Close)
	return m
}

// defaultClipboardOptions are the settings a new install has.
func defaultClipboardOptions() clipboardbuiltin.Options {
	return clipboardbuiltin.Options{RootList: true, RootSearch: true, LinkPreviews: true}
}
