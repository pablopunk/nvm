package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fakeExtensions is an extensionController that records the calls the views make. Its listings are the fixtures
// the tests and goldens state directly, so no child process starts.
type fakeExtensions struct {
	listings []manager.Listing
	sources  map[string]string
	calls    []string
	err      error
}

func (f *fakeExtensions) List() []manager.Listing { return f.listings }

func (f *fakeExtensions) Refresh() error {
	f.calls = append(f.calls, "refresh")
	return nil
}

func (f *fakeExtensions) Enable(id string) error    { return f.record("enable", id) }
func (f *fakeExtensions) Disable(id string) error   { return f.record("disable", id) }
func (f *fakeExtensions) Uninstall(id string) error { return f.record("uninstall", id) }

func (f *fakeExtensions) Source(id string) (string, error) {
	source, ok := f.sources[id]
	if !ok {
		return "", manager.ErrUnknownExtension
	}
	return source, nil
}

func (f *fakeExtensions) record(op, id string) error {
	f.calls = append(f.calls, op+" "+id)
	return f.err
}

// fixedController returns ctl for every search, as a manager that is already running does.
func fixedController(ctl extensionController) func() extensionController {
	return func() extensionController { return ctl }
}

// viewHost is a search.Host that counts what a view asked of the palette: pops and refreshes as well as pushes.
type viewHost struct {
	recordingHost
	pops int
}

// Pop counts the returns from a view.
func (h *viewHost) Pop() { h.pops++ }

const fixtureExtensionsDir = "/nvm-fixture/shared/nevermind/extensions"

func fixtureDir(id string) string { return fixtureExtensionsDir + "/" + id }

// goodListing is an enabled extension that runs.
func goodListing() manager.Listing {
	return manager.Listing{
		ID: "acme.good", Title: "Good fixture", Dir: fixtureDir("acme.good"), Enabled: true,
		Capabilities: []api.Capability{api.CapabilityDesktopApps},
		Runtime:      supervisor.Status{State: supervisor.StateRunning},
	}
}

// brokenListing is an enabled extension whose source does not load.
func brokenListing() manager.Listing {
	return manager.Listing{
		ID: "acme.syntax", Title: "Broken source fixture", Dir: fixtureDir("acme.syntax"), Enabled: true,
		Runtime: supervisor.Status{State: supervisor.StateBroken, Reason: "main.go:7:2: expected ';', found '}'",
			Diagnostics: []string{"does not load: main.go:7:2: expected ';', found '}'"}},
	}
}

// crashListing is an enabled extension the runtime stopped after repeated failures.
func crashListing() manager.Listing {
	return manager.Listing{
		ID: "acme.crash", Title: "Crashing fixture", Dir: fixtureDir("acme.crash"), Enabled: true,
		Runtime: supervisor.Status{State: supervisor.StateDisabled, Failures: 3,
			Reason:      "disabled after 3 failures in 1m0s; last: child exited with status 2",
			Diagnostics: []string{"failed: child exited with status 2", "failed: child exited with status 2"}},
	}
}

// spinListing is a folder that is off and has never run.
func spinListing() manager.Listing {
	return manager.Listing{ID: "acme.spin", Title: "Hanging fixture", Dir: fixtureDir("acme.spin")}
}

// restartingListing is an enabled extension whose last child failed and that waits for its backoff.
func restartingListing() manager.Listing {
	return manager.Listing{
		ID: "acme.slow", Title: "Slow fixture", Dir: fixtureDir("acme.slow"), Enabled: true,
		Runtime: supervisor.Status{State: supervisor.StateFailed, Reason: "child exited with status 2"},
	}
}

// startingListing is an enabled extension whose child has not reported ready yet.
func startingListing() manager.Listing {
	return manager.Listing{
		ID: "acme.initspin", Title: "Slow init fixture", Dir: fixtureDir("acme.initspin"), Enabled: true,
		Runtime: supervisor.Status{State: supervisor.StateStarting},
	}
}

// invalidListing is a folder whose manifest does not read, so it cannot run at all.
func invalidListing() manager.Listing {
	return manager.Listing{
		ID: "acme.bad", Title: "acme.bad", Dir: fixtureDir("acme.bad"),
		Problems: []string{"extension.json is missing or unreadable"},
	}
}

// reviewListing declares one known capability and one this version does not know.
func reviewListing() manager.Listing {
	return manager.Listing{
		ID: "acme.review", Title: "Capability fixture", Dir: fixtureDir("acme.review"),
		Capabilities:        []api.Capability{api.CapabilityDesktopApps, "remote.control"},
		UnknownCapabilities: []api.Capability{"remote.control"},
	}
}

// findRowTitled returns the row of rows whose title is title, failing the test when there is none.
func findRowTitled(t *testing.T, rows []search.Item, title string) search.Item {
	t.Helper()
	for _, row := range rows {
		if row.Title == title {
			return row
		}
	}
	t.Fatalf("no row %q among %d rows", title, len(rows))
	return search.Item{}
}
