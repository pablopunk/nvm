package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/search"
)

// openFixtureView shows the fixture view of ref on the palette the way a pushed page opens: the session starts its
// loads on host, and the page is the palette's page above the root.
func openFixtureView(t *testing.T, p *App, m *manager.Manager, ref string, deps extview.Deps) (*extview.Session, *palette.View, *viewHost) {
	t.Helper()
	return openFixtureViewWithArgs(t, p, m, ref, nil, deps)
}

// openFixtureViewWithArgs is openFixtureView for a view whose fixture reads the invocation's arguments.
func openFixtureViewWithArgs(t *testing.T, p *App, m *manager.Manager, ref string, args map[string]string, deps extview.Deps) (*extview.Session, *palette.View, *viewHost) {
	t.Helper()
	result, failure, err := m.Run(context.Background(), viewFixtureExtension, api.Invocation{Ref: api.ActionRef{ID: ref, Args: args}})
	if err != nil || failure != nil || result.View == nil {
		t.Fatalf("fixture %s: err %v failure %+v", ref, err, failure)
	}
	session := extview.NewSession(deps, viewFixtureExtension, *result.View)
	host := &viewHost{}
	page := session.Open(host)
	goldenOpen(p, page)
	return session, page, host
}

// rowsOf returns the rows the page shows for the empty query.
func rowsOf(page *palette.View) []search.Item {
	return page.Search("")
}

func TestSourceLoadsRefreshesInPlaceAndPaginates(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	_, page, host := openFixtureView(t, p, m, "source", syncViewDeps(m))

	rows := rowsOf(page)
	if got := findRowTitled(t, rows, "Standup notes").Subtitle; got != "Refreshed just now" {
		t.Fatalf("immediate refresh did not patch the loaded row: subtitle %q", got)
	}
	more := rows[len(rows)-1]
	if more.Title != "Load more" {
		t.Fatalf("last row = %q, want the load-more row", more.Title)
	}
	more.Actions[0].Run(host)
	titles := rowLabelsOf(rowsOf(page))
	if !hasTitle(titles, "Retro summary") || !hasTitle(titles, "Hiring plan") {
		t.Fatalf("after load more, rows = %v", titles)
	}
}

func TestSearchAccessoryReplacesTheRowsThroughTheExtension(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	_, page, _ := openFixtureView(t, p, m, "source", syncViewDeps(m))
	if page.Accessory == nil || page.Accessory.Label() != "All" {
		t.Fatalf("accessory = %+v, want the All option shown", page.Accessory)
	}
	page.Accessory.OnChange("done")
	if titles := rowLabelsOf(rowsOf(page)); len(titles) == 0 || titles[0] != "Release plan" {
		t.Fatalf("filtered rows = %v, want only the done note", titles)
	}
}

func TestFailedLoadShowsItsReasonAndARetryRow(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	_, page, _ := openFixtureView(t, p, m, "failing", syncViewDeps(m))
	rows := rowsOf(page)
	failed := findRowTitled(t, rows, "Could not load these items")
	if !failed.Disabled || !strings.Contains(failed.Subtitle, "offline") {
		t.Fatalf("failure row = disabled %v subtitle %q", failed.Disabled, failed.Subtitle)
	}
	findRowTitled(t, rows, "Try again")
}

func TestLoadingSourceShowsNoEmptyState(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	deps := syncViewDeps(m)
	deps.Load = func(string, string, func([]api.Item, *api.Failure)) {}
	_, page, _ := openFixtureView(t, p, m, "source", deps)
	if page.Loading == nil || !page.Loading() {
		t.Fatal("a source that is still loading must not show the empty state")
	}
}

func TestSnapshotShowsBeforeTheReloadAndReloadsWhenStale(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	clock := fake.NewClock()
	cache := extview.NewMemoryCache()
	loads := 0
	deps := syncViewDeps(m)
	deps.Clock, deps.Cache = clock, cache
	deps.Load = func(extID, loader string, done func([]api.Item, *api.Failure)) {
		loads++
		syncViewDeps(m).Load(extID, loader, done)
	}
	openFixtureView(t, p, m, "source", deps)
	if loads != 1 {
		t.Fatalf("first open loads = %d, want 1", loads)
	}
	_, page, _ := openFixtureView(t, p, m, "source", deps)
	if loads != 1 || len(rowsOf(page)) == 0 {
		t.Fatalf("fresh snapshot: loads %d, rows %v; want no load and the snapshot shown", loads, rowLabelsOf(rowsOf(page)))
	}
	clock.Advance(2 * time.Minute)
	_, page, _ = openFixtureView(t, p, m, "source", deps)
	if loads != 2 || len(rowsOf(page)) == 0 {
		t.Fatalf("stale snapshot: loads %d; want the snapshot shown and a reload in place", loads)
	}
}

func TestEmptyViewShowsItsEmptyState(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	_, page, _ := openFixtureView(t, p, m, "empty", syncViewDeps(m))
	if page.EmptyTitle != "No snippets yet" {
		t.Fatalf("empty title = %q", page.EmptyTitle)
	}
}

func TestDisabledRowsHaveNoActions(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	_, page, _ := openFixtureView(t, p, m, "selected", syncViewDeps(m))
	archived := findRowTitled(t, rowsOf(page), "Archived task")
	if !archived.Disabled || len(archived.Actions) != 0 {
		t.Fatalf("disabled row = disabled %v actions %d", archived.Disabled, len(archived.Actions))
	}
}

func TestActionShortcutsReachTheirAction(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	_, page, _ := openFixtureView(t, p, m, "shortcuts", syncViewDeps(m))
	paste := actionTitled(t, findRowTitled(t, rowsOf(page), "Sign-off"), "Paste text")
	if paste.Key != ui.KeyY || paste.Mods != palette.Mod || paste.Label != palette.KeyLabel("⌃")+"Y" {
		t.Fatalf("paste shortcut = key %v mods %v label %q", paste.Key, paste.Mods, paste.Label)
	}
}

func TestCollectionRecordsEditAndRemoveWithCmdShortcuts(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	_, page, _ := openFixtureView(t, p, m, "collection", syncViewDeps(m))
	record := findRowTitled(t, rowsOf(page), "Standup notes")
	edit := actionTitled(t, record, "Edit")
	remove := actionTitled(t, record, "Remove")
	if edit.Key != ui.KeyE || edit.Mods != palette.Mod {
		t.Fatalf("edit shortcut = key %v mods %v, want Cmd/Ctrl+E", edit.Key, edit.Mods)
	}
	if remove.Key != ui.KeyBackspace || remove.Mods != palette.Mod {
		t.Fatalf("remove shortcut = key %v mods %v, want Cmd/Ctrl+Backspace", remove.Key, remove.Mods)
	}
}

func TestRootNavigationRootsThePaletteBeforeOpeningTheView(t *testing.T) {
	m := viewManager(t)
	deps := syncViewDeps(m)
	host := &rootCountingHost{}
	extview.ActionFor(deps, viewFixtureExtension, api.Run("root", "Root", nil)).Run(host)
	if host.roots != 1 || len(host.pushed) != 1 {
		t.Fatalf("root navigation: roots %d pushed %d, want one root and one pushed view", host.roots, len(host.pushed))
	}
}

func TestImagesLoadBeforeTheyDraw(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	_, page, _ := openFixtureView(t, p, m, "images", syncViewDeps(m))
	rowsOf(page)
	if row := findRowTitled(t, rowsOf(page), "Dashboard"); row.Image == nil {
		t.Fatal("the image row has no decoded image after it loaded")
	}
}

func TestImageFilePreviewDecodesTheFile(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	path, err := filepath.Abs(filepath.Join("testdata", "views-fixtures", "dot.png"))
	if err != nil {
		t.Fatal(err)
	}
	_, page, _ := openFixtureViewWithArgs(t, p, m, "image-file", map[string]string{"path": path}, syncViewDeps(m))
	if page.Body == nil {
		t.Fatal("image preview has no body")
	}
}

func TestBoundedImageRefusesOversizedDataURLs(t *testing.T) {
	if bitmap := loadExtensionBitmap(nil, "data:image/png;base64,"+strings.Repeat("A", maxImageDataURLBytes+1)); bitmap != nil {
		t.Fatal("an oversized data URL was decoded")
	}
}

// rootCountingHost is the palette host an action runs against, counting the root returns.
type rootCountingHost struct {
	viewHost
	roots int
}

// Root counts the returns to the root.
func (h *rootCountingHost) Root() { h.roots++ }

// The golden states: each is a fixture view in one state (source list, loading, empty, failure, confirmation, selected
// row, images with a detail).
func TestPaletteViewSourceList(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	openFixtureView(t, p, m, "source", syncViewDeps(m))
	golden.AssertMatrix(t, "source-list", p.View, viewsGoldens)
}

func TestPaletteViewLoadingState(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	deps := syncViewDeps(m)
	deps.Load = func(string, string, func([]api.Item, *api.Failure)) {}
	openFixtureView(t, p, m, "source", deps)
	golden.AssertMatrix(t, "loading", p.View, viewsGoldens)
}

func TestPaletteViewEmptyState(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	openFixtureView(t, p, m, "empty", syncViewDeps(m))
	golden.AssertMatrix(t, "empty", p.View, viewsGoldens)
}

func TestPaletteViewFailureState(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	openFixtureView(t, p, m, "failing", syncViewDeps(m))
	golden.AssertMatrix(t, "failure", p.View, viewsGoldens)
}

func TestPaletteViewConfirmationState(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	_, page, host := openFixtureView(t, p, m, "confirm", syncViewDeps(m))
	_ = page
	findRowTitled(t, rowsOf(page), "Alpha note").Actions[0].Run(host)
	confirm, ok := host.pushed[len(host.pushed)-1].(*palette.View)
	if !ok {
		t.Fatalf("the destructive action pushed %T, want the confirmation page", host.pushed[len(host.pushed)-1])
	}
	goldenOpen(p, confirm)
	golden.AssertMatrix(t, "confirmation", p.View, viewsGoldens)
}

func TestPaletteViewSelectedRow(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	openFixtureView(t, p, m, "selected", syncViewDeps(m))
	golden.AssertMatrix(t, "selected", p.View, viewsGoldens)
}

func TestPaletteViewImagesWithDetail(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	openFixtureView(t, p, m, "images", syncViewDeps(m))
	goldenQuery(p, "")
	p.Choose(0)
	golden.AssertMatrix(t, "images", p.View, viewsGoldens)
}

func hasTitle(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func rowLabelsOf(rows []search.Item) []string {
	titles := make([]string, 0, len(rows))
	for _, row := range rows {
		titles = append(titles, row.Title)
	}
	return titles
}
