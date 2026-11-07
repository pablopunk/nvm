package app

import (
	"context"
	clipboardbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/clipboard"
	windowmanagementbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/windowmanagement"
	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ai/extai"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/features/notes"
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

// newManagerWithApps is newManagerOverNetwork with the apps lifecycle the host hands the services and the clipboard
// ports the extensions read.
func newManagerWithApps(t testing.TB, builtins []api.Extension, clock platform.Clock, network platform.HTTP, apps services.AppLifecycle, extras extensionPorts, enabled ...string) *manager.Manager {
	t.Helper()
	return newManagerWithFakes(t, managerFakes{extras: extras}, builtins, clock, network, apps, enabled...)
}

// managerFakes are the host services a test puts in place of the real ones: the file index, the model calls, the
// signed-in account, the clipboard ports and the palette's own surfaces. A nil field keeps the service absent, as the
// app without it does.
type managerFakes struct {
	onChange       func()
	commandsChange func()
	index          services.FileIndex
	ai             *extai.Service
	account        services.AccountState
	extras         extensionPorts
	settingsHost   func(string) services.SettingsHost
	permissions    services.PermissionSurface
	updates        services.UpdateSurface
	diagnostics    services.DiagnosticsSurface
	jobs           services.JobsSurface
	shortcuts      services.ShortcutTable
}

// paletteSurfaces are the palette's own surfaces as the app hands them to the built-ins: its settings, permissions,
// updates, diagnostics, background jobs and shortcuts.
func paletteSurfaces(p *App) managerFakes {
	return managerFakes{
		settingsHost: p.builtInSettingsStore,
		permissions:  permissionSurface{p: p},
		updates:      updatesSurface{p: p},
		diagnostics:  diagnosticsSurface{p: p},
		jobs:         jobsSurface{p: p},
		shortcuts:    p.extensionShortcuts(),
	}
}

// newFixtureManagerWithSurfaces is newFixtureManager with the palette's apps lifecycle, the fixture's clock and
// clipboard, and the surfaces given, so a golden shows the states its fixture names.
func newFixtureManagerWithSurfaces(t *testing.T, p *App, surfaces managerFakes) *manager.Manager {
	t.Helper()
	return newManagerWithFakes(t, surfaces, builtInExtensions(builtInInputs{Zone: time.UTC, Clipboard: surfaces.extras.options, WindowSettings: windowmanagementbuiltin.DefaultSettings()}), surfaces.extras.clock, nil, p.appsvc)
}

// newFixtureManagerWithApps is newFixtureManagerWithSurfaces with the palette's own surfaces, so the applications
// built-in lists the apps the palette indexed.
func newFixtureManagerWithApps(t *testing.T, p *App) *manager.Manager {
	t.Helper()
	return newFixtureManagerWithSurfaces(t, p, paletteSurfaces(p))
}

// newFixtureManagerWithDiagnostics is newFixtureManagerWithApps with the support records given, so a golden can show the
// records its fixture names.
func newFixtureManagerWithDiagnostics(t *testing.T, p *App, diagnostics services.DiagnosticsSurface) *manager.Manager {
	t.Helper()
	surfaces := paletteSurfaces(p)
	surfaces.diagnostics = diagnostics
	return newFixtureManagerWithSurfaces(t, p, surfaces)
}

// newFixtureManagerWithJobs is newFixtureManagerWithApps with the background jobs given, so a golden can show the jobs its
// fixture names.
func newFixtureManagerWithJobs(t *testing.T, p *App, jobs services.JobsSurface) *manager.Manager {
	t.Helper()
	surfaces := paletteSurfaces(p)
	surfaces.jobs = jobs
	return newFixtureManagerWithSurfaces(t, p, surfaces)
}

// newFixtureManagerWithUpdates is newFixtureManagerWithApps with the updates surface given, so a golden can show a state
// the host does not have.
func newFixtureManagerWithUpdates(t *testing.T, p *App, updates services.UpdateSurface) *manager.Manager {
	t.Helper()
	surfaces := paletteSurfaces(p)
	surfaces.updates = updates
	return newFixtureManagerWithSurfaces(t, p, surfaces)
}

// newManagerWithFakes is newManagerWithApps with the host services of fakes.
func newManagerWithFakes(t testing.TB, fakes managerFakes, builtins []api.Extension, clock platform.Clock, network platform.HTTP, apps services.AppLifecycle, enabled ...string) *manager.Manager {
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
		Files: ports.FS, FileIndex: fakes.index, AI: fakes.ai, Account: fakes.account, Clock: clock, Worker: real.NewWorker(), Processes: ports.Processes,
		Capabilities:    platform.NewCapabilities(platform.AllCapabilities()...),
		Places:          fixturePlaces,
		Network:         network,
		Apps:            apps,
		History:         fakes.extras.history,
		Chats:           fakes.extras.chats,
		Builder:         fakes.extras.builder,
		ClipboardImages: fakes.extras.images,
		Notes:           fakes.extras.notes,
		Permissions:     fakes.permissions,
		Updates:         fakes.updates,
		Diagnostics:     fakes.diagnostics,
		Jobs:            fakes.jobs,
		Shortcuts:       fakes.shortcuts,
	}, filepath.Join(dir, "extensions"), enablement.Open(state), manager.Options{
		Executable:      exe,
		DataRoot:        filepath.Join(dir, "extension-data"),
		Home:            userHomeDir(),
		BuiltIns:        builtins,
		BuiltInSettings: fakes.settingsHost,
		OnChange:        fakes.onChange,
		CommandsChange:  fakes.commandsChange,
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
	m.Commands(context.Background()) // Prime deterministic fixtures; production discovers commands asynchronously.
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

// fixtureRestartPoll is how often waitForState calls the extensions while a failed child waits out its backoff.
const fixtureRestartPoll = 25 * time.Millisecond

// waitForState blocks until extension id reports the state want, or fails the test. A child that failed to start
// (a start that missed its budget under load, or a child that died) restarts only on a call once its backoff has
// passed, as the palette's own calls restart it, so the wait makes that call rather than waiting for a state no one
// will reach.
func waitForState(t *testing.T, m *manager.Manager, id string, want supervisor.State) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), fixtureWaitTimeout)
	defer cancel()
	for {
		if err := m.WaitFor(ctx, id, func(s supervisor.Status) bool { return s.State == want || s.State == supervisor.StateFailed }); err != nil {
			t.Fatalf("waiting for %s to be %s: %v; last status %+v", id, want, err, listingFor(t, m, id).Runtime)
		}
		status := listingFor(t, m, id).Runtime
		if status.State == want {
			return
		}
		_ = m.Commands(ctx)
		time.Sleep(fixtureRestartPoll)
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
	clock   platform.Clock
	notes   func() *notes.Store
	chats   services.ChatReader
	builder services.BuilderPort
}

// pinnedNowClock is the real clock with its instant pinned, so the relative times a golden draws are the same every run.
type pinnedNowClock struct {
	platform.Clock
	now time.Time
}

// Now is the pinned instant.
func (c pinnedNowClock) Now() time.Time { return c.now }

// goldenClock is the clock the goldens' extensions read: the real one with the golden instant pinned.
func goldenClock() platform.Clock {
	return pinnedNowClock{Clock: real.NewPorts().Clock, now: goldenNow}
}

// testExtensionPorts are the clipboard history and pictures the palette's extension services read, for the golden
// palette's manager. They follow the palette's clipboard engine as it changes.
func (p *App) testExtensionPorts() extensionPorts {
	return extensionPorts{history: p.extensionHistory, images: p.extensionClipboardImages(), options: p.clipboardOptions, clock: goldenClock(),
		chats: extensionChats{p: p}, builder: extensionBuilder{p: p}}
}
