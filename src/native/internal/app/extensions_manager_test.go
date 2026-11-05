package app

import (
	clipboardbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/clipboard"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/store"
)

const fixtureWaitTimeout = 20 * time.Second

// newFixtureManager opens a manager on a temp folder with a real store, and installs the named fixtures enabled.
// Their children are this test binary (see TestMain).
func newFixtureManager(t *testing.T, enabled ...string) *manager.Manager {
	t.Helper()
	return newManagerOver(t, builtInExtensions(builtInInputs{Zone: time.UTC}), enabled...)
}

// newManagerOver is newFixtureManager with the built-ins it runs given: a benchmark passes only the one it measures.
func newManagerOver(t testing.TB, builtins []api.Extension, enabled ...string) *manager.Manager {
	t.Helper()
	return newManagerOverNetwork(t, builtins, nil, nil, enabled...)
}

// newManagerOverNetwork is newManagerOver with the clock and the public network port the extensions use. A nil clock
// is the real one; a nil network refuses every fetch.
func newManagerOverNetwork(t testing.TB, builtins []api.Extension, clock platform.Clock, network platform.HTTP, enabled ...string) *manager.Manager {
	return newManagerWithApps(t, builtins, clock, network, nil, extensionPorts{}, enabled...)
}

// newFixtureManagerWithApps is newFixtureManager with the palette's apps lifecycle, so the applications built-in lists
// the apps the palette indexed.
func newFixtureManagerWithApps(t *testing.T, apps services.AppLifecycle, extras extensionPorts) *manager.Manager {
	t.Helper()
	return newManagerWithApps(t, builtInExtensions(builtInInputs{Zone: time.UTC, Clipboard: extras.options}), nil, nil, apps, extras)
}

// newManagerWithApps is newManagerOverNetwork with the apps lifecycle the host hands the services.
func newManagerWithApps(t testing.TB, builtins []api.Extension, clock platform.Clock, network platform.HTTP, apps services.AppLifecycle, extras extensionPorts, enabled ...string) *manager.Manager {
	t.Helper()
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
	if clock == nil {
		clock = ports.Clock
	}
	m, err := manager.New(manager.Ports{
		Files: ports.FS, Clock: clock, Worker: real.NewWorker(), Processes: ports.Processes,
		Capabilities: platform.NewCapabilities(platform.AllCapabilities()...),
		Places:       fixturePlaces,
		Network:      network,
		Apps:         apps,
		History:      extras.history,
		ClipboardImages: extras.images,
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
	for _, id := range enabled {
		if err := m.Install(id, fixtureFiles(t, id), true); err != nil {
			t.Fatalf("install %s: %v", id, err)
		}
	}
	return m
}

// fixtureFiles reads an extension folder from the extension fixtures.
func fixtureFiles(t testing.TB, id string) map[string][]byte {
	t.Helper()
	dir := filepath.Join("..", "ext", "testdata", "extensions", id)
	files := map[string][]byte{}
	for _, name := range []string{bundle.ManifestFile, bundle.SourceFile} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s of %s: %v", name, id, err)
		}
		files[name] = data
	}
	return files
}

// waitForState blocks until extension id reports the state want, or fails the test.
func waitForState(t *testing.T, m *manager.Manager, id string, want supervisor.State) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), fixtureWaitTimeout)
	defer cancel()
	if err := m.WaitFor(ctx, id, func(s supervisor.Status) bool { return s.State == want }); err != nil {
		t.Fatalf("waiting for %s to be %s: %v", id, want, err)
	}
}

func listingFor(t *testing.T, m *manager.Manager, id string) manager.Listing {
	t.Helper()
	for _, l := range m.List() {
		if l.ID == id {
			return l
		}
	}
	t.Fatalf("no listing for %s", id)
	return manager.Listing{}
}

// fixturePlaces lists the standard folders of the process's home, as the app's system service lists them.
func fixturePlaces() []api.Place {
	home, _ := os.UserHomeDir()
	return apiPlaces(systemfeat.Places(home))
}

// extensionPorts are the clipboard history and pictures a test manager's services read. A zero value serves none.
type extensionPorts struct {
	history func() *clipboard.History
	images  services.ClipboardImages
	options func() clipboardbuiltin.Options
}

// testExtensionPorts are the clipboard history and pictures the palette's extension services read, for the golden
// palette's manager. They follow the palette's clipboard engine as it changes.
func (p *App) testExtensionPorts() extensionPorts {
	return extensionPorts{history: p.extensionHistory, images: p.extensionClipboardImages(), options: p.clipboardOptions}
}
