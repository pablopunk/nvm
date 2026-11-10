package manager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/child"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/store"
)

// TestMain makes the test binary serve as the extension child when the manager starts one.
func TestMain(m *testing.M) {
	if code, handled := child.Main(os.Args); handled {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

const (
	waitTimeout = 20 * time.Second
	// searchBudget keeps hanging fixtures from slowing the suite; production uses the B8 budget of 100 ms.
	searchBudget = 300 * time.Millisecond
)

// harness is one manager on a real folder, with a real store for the enabled state.
type harness struct {
	t       *testing.T
	m       *Manager
	root    string
	files   platform.FS
	enabled *enablement.Set
}

func newHarness(t *testing.T, tweak ...func(*Options)) *harness {
	t.Helper()
	dir := t.TempDir()
	ports := real.NewPorts()
	root := filepath.Join(dir, "extensions")
	s, err := store.Open(ports.FS, ports.Clock, filepath.Join(dir, "state"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	set := enablement.Open(s)
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	opts := Options{
		Executable: exe,
		DataRoot:   filepath.Join(dir, "extension-data"),
		Budgets: map[string]supervisor.Budget{
			protocol.MethodSearch: {Soft: 50 * time.Millisecond, Hard: searchBudget},
		},
	}
	for _, change := range tweak {
		change(&opts)
	}
	m, err := New(Ports{
		Files:     ports.FS,
		Clock:     ports.Clock,
		Worker:    real.NewWorker(),
		Processes: ports.Processes,
	}, root, set, opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(m.Close)
	return &harness{t: t, m: m, root: root, files: ports.FS, enabled: set}
}

func TestNewRunObserverObservesRun(t *testing.T) {
	var outcomes []RunOutcome
	observer := recordingRunObserver{record: func(outcome RunOutcome) { outcomes = append(outcomes, outcome) }}
	h := newHarness(t, func(opts *Options) { opts.RunObserver = observer })
	if err := h.m.Install("acme.settings", fixtureContents(t, "acme.settings"), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	h.waitRunning("acme.settings")
	_, failure, err := h.m.Run(context.Background(), "acme.settings", api.Invocation{Ref: api.ActionRef{ID: "open"}})
	if err != nil || failure != nil {
		t.Fatalf("Run: err %v failure %+v", err, failure)
	}
	if len(outcomes) != 1 || outcomes[0] != RunOutcomeSuccess {
		t.Fatalf("observed outcomes = %v, want [success]", outcomes)
	}
}

type recordingRunObserver struct{ record func(RunOutcome) }

func (o recordingRunObserver) BeginRun(ctx context.Context) (context.Context, func(RunOutcome)) {
	return ctx, o.record
}

// fixtureContents reads a fixture folder from testdata, optionally with a different main.go.
func fixtureContents(t *testing.T, name string, source ...string) map[string][]byte {
	t.Helper()
	dir := filepath.Join("..", "testdata", "extensions", name)
	manifest, err := os.ReadFile(filepath.Join(dir, bundle.ManifestFile))
	if err != nil {
		t.Fatalf("read manifest of %s: %v", name, err)
	}
	code := []byte(nil)
	if len(source) > 0 {
		code = []byte(source[0])
	} else {
		code, err = os.ReadFile(filepath.Join(dir, bundle.SourceFile))
		if err != nil {
			t.Fatalf("read source of %s: %v", name, err)
		}
	}
	return map[string][]byte{bundle.ManifestFile: manifest, bundle.SourceFile: code}
}

func (h *harness) waitRunning(id string) {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	if err := h.m.WaitFor(ctx, id, func(s supervisor.Status) bool { return s.State == supervisor.StateRunning }); err != nil {
		h.t.Fatalf("waiting for %s to run: %v", id, err)
	}
}

func (h *harness) search(query string) []SearchResult {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	return h.m.Search(ctx, query)
}

func resultFor(results []SearchResult, id string) *SearchResult {
	for i := range results {
		if results[i].ExtensionID == id {
			return &results[i]
		}
	}
	return nil
}

func (h *harness) listing(id string) Listing {
	h.t.Helper()
	for _, listing := range h.m.List() {
		if listing.ID == id {
			return listing
		}
	}
	h.t.Fatalf("no listing for %s in %+v", id, h.m.List())
	return Listing{}
}

func TestDroppedInExtensionDefaultsToDisabledAndDoesNotRun(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	listing := h.listing("acme.good")
	if listing.Enabled || listing.Runtime.State != supervisor.StateIdle {
		t.Fatalf("listing = %+v; want disabled and idle", listing)
	}
	if results := h.search("a"); len(results) != 0 {
		t.Fatalf("disabled extension answered search: %+v", results)
	}
}

func TestEnableStartsTheExtensionAndSearchReturnsItsItems(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := h.m.Enable("acme.good"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	h.waitRunning("acme.good")
	result := resultFor(h.search("a"), "acme.good")
	if result == nil || result.Failure != nil || len(result.Items) != 3 {
		t.Fatalf("search result = %+v; want three items", result)
	}
}

func TestHostileExtensionDoesNotBlockTheOthers(t *testing.T) {
	h := newHarness(t)
	for _, id := range []string{"acme.good", "acme.spin"} {
		if err := h.m.Install(id, fixtureContents(t, id), true); err != nil {
			t.Fatalf("Install %s: %v", id, err)
		}
	}
	h.waitRunning("acme.good")
	h.waitRunning("acme.spin")
	clock := real.NewPorts().Clock
	start := clock.Now()
	results := h.search("a")
	if elapsed := clock.Now().Sub(start); elapsed > 2*time.Second {
		t.Fatalf("search took %s with a hanging extension; want it bounded by the budget", elapsed)
	}
	good := resultFor(results, "acme.good")
	if good == nil || good.Failure != nil || len(good.Items) != 3 {
		t.Fatalf("good extension result = %+v; want its items despite the hang", good)
	}
	spin := resultFor(results, "acme.spin")
	if spin == nil || spin.Failure == nil || spin.Failure.Code != api.CodeTimeout {
		t.Fatalf("spin result = %+v; want a timeout failure", spin)
	}
}

func TestReinstallHotReloadsAndAFailedInstallKeepsTheRunningVersion(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	h.waitRunning("acme.good")
	before := h.listing("acme.good").Runtime.State
	if before != supervisor.StateRunning {
		t.Fatalf("state = %s; want running", before)
	}

	reloaded := fixtureContents(t, "acme.good", "package main\n\nimport \"github.com/pablopunk/nvm/native/internal/ext/api\"\n\nfunc SearchItems(ctx api.Context, query string) ([]api.Item, error) {\n\treturn []api.Item{{ID: \"v2\", Title: \"v2\"}}, nil\n}\n")
	if err := h.m.Install("acme.good", reloaded, true); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	h.waitRunning("acme.good")
	if result := resultFor(h.search("x"), "acme.good"); result == nil || len(result.Items) != 1 || result.Items[0].ID != "v2" {
		t.Fatalf("after hot reload result = %+v; want the v2 item", result)
	}

	wrongID := fixtureContents(t, "acme.good")
	wrongID[bundle.ManifestFile] = []byte(`{"apiVersion":"1.0","id":"acme.other","title":"Other"}`)
	if err := h.m.Install("acme.good", wrongID, true); err == nil {
		t.Fatal("an install naming another id succeeded")
	}
	if result := resultFor(h.search("x"), "acme.good"); result == nil || len(result.Items) != 1 || result.Items[0].ID != "v2" {
		t.Fatalf("after failed install result = %+v; want the v2 item still running", result)
	}
}

func TestUninstallStopsTheRuntimeAndRemovesTheFolder(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	h.waitRunning("acme.good")
	if err := h.m.Uninstall("acme.good"); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if listings := h.m.List(); len(listings) != 0 {
		t.Fatalf("listings after uninstall = %+v", listings)
	}
	if _, err := h.files.ReadFile(filepath.Join(h.root, "acme.good", bundle.ManifestFile)); err == nil {
		t.Fatal("folder still on disk after uninstall")
	}
	if results := h.search("a"); len(results) != 0 {
		t.Fatalf("uninstalled extension answered: %+v", results)
	}
}

func TestEnableAndDisableRaceWithSearchAndRefresh(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for round := 0; round < 15; round++ {
				switch (worker + round) % 4 {
				case 0:
					_ = h.m.Enable("acme.good")
				case 1:
					_ = h.m.Disable("acme.good")
				case 2:
					_ = h.m.Refresh()
				default:
					_ = h.m.Search(context.Background(), "a")
				}
			}
		}(worker)
	}
	wg.Wait()

	if err := h.m.Disable("acme.good"); err != nil {
		t.Fatalf("final Disable: %v", err)
	}
	if listing := h.listing("acme.good"); listing.Enabled || listing.Runtime.State != supervisor.StateIdle {
		t.Fatalf("after races and disable: %+v; want disabled and idle", listing)
	}
	if err := h.m.Enable("acme.good"); err != nil {
		t.Fatalf("final Enable: %v", err)
	}
	h.waitRunning("acme.good")
}

func TestBrokenSourceIsListedWithItsReason(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.syntax", fixtureContents(t, "acme.syntax"), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	if err := h.m.WaitFor(ctx, "acme.syntax", func(s supervisor.Status) bool { return s.State == supervisor.StateBroken }); err != nil {
		t.Fatalf("waiting for broken: %v", err)
	}
	listing := h.listing("acme.syntax")
	if !strings.Contains(listing.Runtime.Reason, "compile") {
		t.Fatalf("reason = %q; want the compile error", listing.Runtime.Reason)
	}
}

func TestReviewDataAndSourceAreAvailableBeforeAnythingRuns(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), false); err != nil {
		t.Fatalf("Install: %v", err)
	}
	listing := h.listing("acme.good")
	if len(listing.Capabilities) != 1 || listing.Capabilities[0] != api.CapabilityDesktopApps {
		t.Fatalf("capabilities = %v; want desktop.apps", listing.Capabilities)
	}
	source, err := h.m.Source("acme.good")
	if err != nil || !strings.Contains(source, "func SearchItems") {
		t.Fatalf("Source = %q, %v; want the extension source", source, err)
	}
}

func TestMalformedFolderIsListedWithProblemsAndNeverStarts(t *testing.T) {
	h := newHarness(t)
	folder := filepath.Join(h.root, "acme.bad")
	if err := h.files.MkdirAll(folder, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := h.files.WriteFile(filepath.Join(folder, bundle.ManifestFile), []byte(`{"id":`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := h.m.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	listing := h.listing("acme.bad")
	if len(listing.Problems) == 0 || listing.Runtime.State != "" {
		t.Fatalf("listing = %+v; want problems and no runtime", listing)
	}
	if err := h.m.Enable("acme.bad"); err == nil {
		t.Fatal("Enable of a folder that does not load succeeded")
	}
}

// TestRuntimeStateChangeNotifiesTheOwner is the signal a view waits for to redraw: a child that becomes ready
// reports a change, so the Extensions list shows Running without being reopened.
func TestRuntimeStateChangeNotifiesTheOwner(t *testing.T) {
	changes := make(chan struct{}, 16)
	h := newHarness(t, func(o *Options) {
		o.OnChange = func() {
			select {
			case changes <- struct{}{}:
			default:
			}
		}
	})
	if err := h.m.Install("acme.views", fixtureContents(t, "acme.views"), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	h.waitRunning("acme.views")
	select {
	case <-changes:
	case <-time.After(waitTimeout):
		t.Fatal("no state change reported for a child that became ready")
	}
}
