package extview

import (
	"context"
	"crypto/sha256"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"path/filepath"
	"testing"
)

func TestLoaderReadDenialFiltersMatchingCachedSourceOnly(t *testing.T) {
	for _, newer := range []bool{false, true} {
		t.Run(map[bool]string{false: "matching", true: "newer-snapshot"}[newer], func(t *testing.T) {
			badPath, goodPath := filepath.Join(t.TempDir(), "bad.png"), filepath.Join(t.TempDir(), "good.png")
			bad, good := scopedTestPicture(badPath), scopedTestPicture(goodPath)
			evidence := &api.SnapshotEvidence{ThumbnailHashes: make(map[[32]byte]bool), ThumbnailPaths: make(map[[32]byte][]string)}
			for ref, path := range map[string]string{bad: badPath, good: goodPath} {
				hash := sha256.Sum256([]byte(ref))
				evidence.ThumbnailHashes[hash] = true
				evidence.ThumbnailPaths[hash] = []string{path}
			}
			items := []api.Item{{ID: badPath, Image: &api.Image{Src: bad}, Primary: api.Primary(api.OpenPath("Open", badPath))}, {ID: goodPath, Image: &api.Image{Src: good}, Primary: api.Primary(api.OpenPath("Open", goodPath))}}
			identity := new(byte)
			cache := NewMemoryCache()
			var complete func(GridImageResult)
			deps := Deps{Clock: fake.NewClock(), Cache: cache, CommandSnapshotIdentity: func(string) any { return identity }, Load: func(_ string, _ string, done func([]api.Item, *api.SnapshotEvidence, *api.Failure)) {
				done(items, evidence, nil)
			}, LoadGridImage: func(_ context.Context, r GridImageRequest, done func(GridImageResult)) {
				if _, ok := r.Scope.Path(r.Binding(), r.Source); !ok {
					t.Fatal("loader evidence was lost")
				}
				complete = done
			}}
			view := api.GridView("Loader", 0)
			view.Source = &api.DataSource{Loader: "pictures", CacheKey: "pictures"}
			s := NewSession(deps, "files", view)
			s.Open(&fakeHost{})
			s.bitmap(&api.Image{Src: bad})
			key := "files/pictures"
			before, _ := cache.Get(key)
			if newer {
				next := before
				next.PictureVersion = new(byte)
				cache.Put(key, next)
			}
			complete(GridImageResult{Denial: &api.SnapshotDenial{Path: badPath}})
			if s.items[0].Image != nil || s.items[0].Primary != nil || s.items[1].Image == nil || s.items[1].Primary == nil {
				t.Fatal("loader denial crossed row authority")
			}
			after, ok := cache.Get(key)
			if !ok || !after.At.Equal(before.At) {
				t.Fatal("path denial removed or renewed source snapshot")
			}
			if newer {
				if after.Items[0].Image == nil {
					t.Fatal("old view changed a newer cached source")
				}
			} else if after.Items[0].Image != nil || after.Items[0].Primary != nil || after.Items[1].Image == nil {
				t.Fatal("cached source retained denied content or lost independent row")
			}
		})
	}
}
