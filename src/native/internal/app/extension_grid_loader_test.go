package app

import (
	"bytes"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/enablement"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
	"image"
	"image/png"
	"path/filepath"
	"testing"
	"time"
)

type gridLoaderProbe struct{ path string }

func (gridLoaderProbe) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.loader-picture", Title: "Loader pictures"}, Capabilities: []api.Capability{api.CapabilityDesktopFiles}}
}
func (p gridLoaderProbe) LoadItems(ctx api.Context, _ string) ([]api.Item, error) {
	ref, found := ctx.Desktop().Files().Thumbnail(p.path)
	if !found {
		return nil, api.ErrNotFound
	}
	return []api.Item{{ID: p.path, Title: "Loaded picture", Image: &api.Image{Src: ref}}}, nil
}

func TestLoaderBackedGridRendersThroughHostEvidenceAndWarmSnapshot(t *testing.T) {
	services := fake.NewSet()
	path := filepath.Join(t.TempDir(), "image.png")
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	services.FS.Seed(path, encoded.Bytes())
	state, err := store.Open(services.FS, services.Clock, "/loader-state")
	if err != nil {
		t.Fatal(err)
	}
	defer state.Close()
	probe := gridLoaderProbe{path: path}
	m, err := manager.New(manager.Ports{Files: services.FS, Clock: services.Clock}, "/loader-extensions", enablement.Open(state), manager.Options{BuiltIns: []api.Extension{probe}})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	queue := make(completionUIQueue, 8)
	p := &App{ports: services.Ports(), uiQueue: queue, files: newTestFiles(t, "/home", nil)}
	deps := p.extensionViewDeps(m)
	deps.Clock, deps.Cache = services.Clock, extview.NewMemoryCache()
	nextUI := func() {
		t.Helper()
		select {
		case fn := <-queue:
			fn()
		case <-time.After(5 * time.Second):
			t.Fatal("loader or image completion missing")
		}
	}
	view := api.GridView("Loader pictures", 0)
	view.Source = &api.DataSource{Loader: "pictures", CacheKey: "pictures"}
	s := extview.NewSession(deps, probe.Manifest().ID, view)
	page := s.Open(&recordingHost{})
	if rows := page.Search(""); len(rows) != 0 {
		t.Fatal("loader grid started with unexpected content")
	}
	nextUI()
	rows := page.Search("")
	if len(rows) != 1 {
		t.Fatal("loader did not install its row")
	}
	nextUI()
	nextUI()
	if rows = page.Search(""); rows[0].Image == nil {
		t.Fatal("valid loader thumbnail was rejected")
	}
	page.OnLeave()
	cached, ok := deps.Cache.Get(probe.Manifest().ID + "/pictures")
	if !ok || cached.PictureEvidence == nil || len(cached.PictureEvidence.ThumbnailHashes) != 1 {
		t.Fatal("source snapshot dropped host evidence")
	}
	warm := extview.NewSession(deps, probe.Manifest().ID, view).Open(&recordingHost{})
	if rows := warm.Search(""); len(rows) != 1 {
		t.Fatal("warm source grid lost its immediate row")
	}
	nextUI()
	nextUI()
	if warm.Search("")[0].Image == nil {
		t.Fatal("warm snapshot did not reauthorize its thumbnail")
	}
	warm.OnLeave()
	p.settleExtensionCalls()
}
