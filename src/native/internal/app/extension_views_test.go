package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/search"
)

// viewFixtureExtension is the dev fixture that returns every view type (internal/ext/testdata/extensions/acme.views).
const viewFixtureExtension = "acme.views"

// viewsGoldens is the group of the view goldens under testdata/golden/views.
var viewsGoldens = golden.Options{Group: "views", Width: width, Height: height}

// syncViewDeps runs extension actions on the calling goroutine, so a test reads the result it caused at once.
func syncViewDeps(m *manager.Manager) extview.Deps {
	return extview.Deps{
		Invoke: func(extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
			result, failure, err := m.Run(context.Background(), extID, in)
			if err != nil {
				failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
			}
			done(result, failure)
		},
		Load: func(extID, loader string, done func([]api.Item, *api.SnapshotEvidence, *api.Failure)) {
			items, evidence, failure, err := m.Load(context.Background(), extID, loader)
			if err != nil {
				failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
			}
			done(items, evidence, failure)
		},
		LoadImage: func(ctx context.Context, ref string, done func(*ui.Bitmap)) {
			done(loadExtensionBitmap(real.NewPorts().FS, ref))
		},
		OpenURL:  func(string) {},
		CopyText: func(search.Host, string) {},
		Files:    real.NewPorts().FS,
	}
}

// fixtureView asks the fixture extension for the view of ref, the way the palette does when the user runs its command.
func fixtureView(t *testing.T, m *manager.Manager, ref string, args map[string]string) *extview.Session {
	t.Helper()
	result, failure, err := m.Run(context.Background(), viewFixtureExtension, api.Invocation{Ref: api.ActionRef{ID: ref, Args: args}})
	if err != nil || failure != nil {
		t.Fatalf("run %s: err %v failure %+v", ref, err, failure)
	}
	if result.View == nil {
		t.Fatalf("run %s returned no view", ref)
	}
	return extview.NewSession(syncViewDeps(m), viewFixtureExtension, *result.View)
}

func viewManager(t *testing.T) *manager.Manager {
	t.Helper()
	m := newFixtureManager(t, viewFixtureExtension)
	waitForState(t, m, viewFixtureExtension, supervisor.StateRunning)
	return m
}

func TestFixtureViewsDrawThroughTheChild(t *testing.T) {
	m := viewManager(t)
	list := fixtureView(t, m, "list", nil).Palette()
	if rows := list.Search(""); len(rows) != 3 || rows[0].Accessory != "Draft +1" || rows[0].Detail == nil {
		t.Fatalf("list rows = %d, first accessory %q", len(rows), rowAccessory(rows))
	}
	if sections := fixtureView(t, m, "sections", nil).Palette().Search(""); sections[0].Section != "Open" {
		t.Fatalf("sections first row section = %q", sections[0].Section)
	}
	if grid := fixtureView(t, m, "grid", nil).Palette(); grid.Grid == nil || grid.Grid.Columns != 3 {
		t.Fatalf("grid = %+v", grid.Grid)
	}
	if rows := fixtureView(t, m, "collection", nil).Palette().Search(""); rows[0].Title != "New snippet" {
		t.Fatalf("collection first row = %q", rows[0].Title)
	}
	if page := fixtureView(t, m, "preview", nil).Palette(); page.Body == nil {
		t.Fatal("preview has no body")
	}
	if page := fixtureView(t, m, "progress", nil).Palette(); page.Body == nil {
		t.Fatal("progress has no body")
	}
}

func rowAccessory(rows []search.Item) string {
	if len(rows) == 0 {
		return ""
	}
	return rows[0].Accessory
}

func TestFixtureFilePreviewReadsTheGivenFile(t *testing.T) {
	m := viewManager(t)
	session := fixtureView(t, m, "file", map[string]string{"path": sampleTextFile(t)})
	if page := session.Palette(); page.Body == nil {
		t.Fatal("file preview has no body")
	}
}

func TestFixtureMarkAsReviewedPatchesTheRowThroughTheChild(t *testing.T) {
	m := viewManager(t)
	session := fixtureView(t, m, "list", nil)
	rows := session.Palette().Search("")
	h := &viewHost{}
	reviewed := findRowTitled(t, rows, "Alpha note")
	actionTitled(t, reviewed, "Mark as reviewed").Run(h)
	if h.refresh != 1 {
		t.Fatalf("patch did not refresh the view (refresh %d)", h.refresh)
	}
	if got := findRowTitled(t, session.Palette().Search(""), "Alpha note"); got.Subtitle != "Reviewed" {
		t.Fatalf("subtitle after the patch = %q, want Reviewed", got.Subtitle)
	}
}

// showView opens a fixture page the way the palette does: as the page above the root, with an empty query.
func showView(t *testing.T, p *App, session *extview.Session) *palette.View {
	t.Helper()
	page := session.Palette()
	goldenOpen(p, page)
	return page
}

func TestPaletteViewListWithDetail(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	showView(t, p, fixtureView(t, m, "list", nil))
	p.Choose(0)
	golden.AssertMatrix(t, "list-detail", p.View, viewsGoldens)
}

func TestPaletteViewSections(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	showView(t, p, fixtureView(t, m, "sections", nil))
	golden.AssertMatrix(t, "sections", p.View, viewsGoldens)
}

func TestPaletteViewGrid(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	showView(t, p, fixtureView(t, m, "grid", nil))
	golden.AssertMatrix(t, "grid", p.View, viewsGoldens)
}

func TestPaletteViewCollection(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	showView(t, p, fixtureView(t, m, "collection", nil))
	p.Choose(1)
	golden.AssertMatrix(t, "collection", p.View, viewsGoldens)
}

func TestPaletteViewPreview(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	showView(t, p, fixtureView(t, m, "preview", nil))
	golden.AssertMatrix(t, "preview", p.View, viewsGoldens)
}

func TestPaletteViewProgress(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	showView(t, p, fixtureView(t, m, "progress", nil))
	golden.AssertMatrix(t, "progress", p.View, viewsGoldens)
}

func TestPaletteViewFilePreview(t *testing.T) {
	m := viewManager(t)
	p := goldenPalette(t)
	showView(t, p, fixtureView(t, m, "file", map[string]string{"path": sampleTextFile(t)}))
	golden.AssertMatrix(t, "file-preview", p.View, viewsGoldens)
}

// sampleTextFile is a text file in testdata, so the file preview golden reads the same bytes on every machine.
func sampleTextFile(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("testdata", "views-fixtures", "sample.txt"))
	if err != nil {
		t.Fatalf("sample file: %v", err)
	}
	return path
}
