package app

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	appsbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/apps"
	appfeat "github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// silentAppSearch is the applications built-in whose search never answers in time: it lists its root catalog, and
// every search call fails as a timed-out call does.
type silentAppSearch struct{ appsbuiltin.Extension }

func (silentAppSearch) SearchItems(api.Context, string) ([]api.Item, error) {
	return nil, errors.New("search timed out")
}

const loomPath = "/usr/share/applications/loom.desktop"

// silentAppsPalette is a palette with Loom installed, whose app search fails, after the root catalog was listed.
func silentAppsPalette(t *testing.T) (*App, *rootInput) {
	t.Helper()
	p := goldenPalette(t)
	p.appsvc = newFixtureAppServices(append(goldenApps(), appfeat.App{Name: "Loom", ID: "com.loom.desktop", Path: loomPath}))
	p.exts.mgr.Store(newManagerWithApps(t, []api.Extension{silentAppSearch{appsbuiltin.New()}}, nil, nil, p.appsvc, extensionPorts{}))
	in := &rootInput{}
	if rows := extensionProvider(p, in).Root(context.Background()); !slices.ContainsFunc(rows, isLoom) {
		t.Fatalf("root catalog does not list Loom: %v", rowTitlesOf(rows))
	}
	return p, in
}

func isLoom(row search.Item) bool { return row.ID == loomPath }

// TestFailedAppSearchKeepsInstalledCatalogMatches is a cold query whose app source fails: the installed Loom still
// matches "loom", so the catalog row stands for the missing answer instead of the app disappearing.
func TestFailedAppSearchKeepsInstalledCatalogMatches(t *testing.T) {
	p, in := silentAppsPalette(t)
	var rows []search.Item
	extensionProvider(p, in).Search(context.Background(), "loom", func(batch search.Batch) {
		if batch.Replace {
			rows = slices.Clone(batch.Items)
		}
	})
	if !slices.ContainsFunc(rows, isLoom) {
		t.Fatalf("failed app search dropped Loom: %v", rowTitlesOf(rows))
	}
}

// TestLoomStaysFirstWhileNarrowingWithASilentAppSource types "lo" then "loo" on the root's single-snapshot search,
// with a command that matches "loo" exactly and an app source that never answers. Loom led "lo" and still matches,
// so it leads "loo".
func TestLoomStaysFirstWhileNarrowingWithASilentAppSource(t *testing.T) {
	p, in := silentAppsPalette(t)
	loo := search.NewRowsProvider("commands", func(_ context.Context, q string) []search.Item {
		if q == "loo" {
			return []search.Item{{ID: "loo", Title: "Loo", Kind: search.KindCommands}}
		}
		return nil
	})
	c := search.NewCoordinator(search.Options{Clock: fake.NewClock(), Synchronous: true, Flat: true, Tiered: true, SingleSnapshot: true})
	providers := []search.Provider{loo, extensionProvider(p, in)}
	for _, q := range []string{"lo", "loo"} {
		snap, _ := c.Query(q, providers)
		if len(snap.Items) == 0 || !isLoom(snap.Items[0]) {
			t.Fatalf("%q rows = %v, want Loom first", q, rowTitlesOf(snap.Items))
		}
	}
}
