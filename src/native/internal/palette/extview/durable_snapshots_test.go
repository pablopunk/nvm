package extview

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/store"
)

func durableGridFixture(t *testing.T) (string, api.ActionRef, api.View) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "image.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	ref := api.ActionRef{ID: "grid"}
	view := api.View{Kind: api.ViewGrid, Refresh: &api.Refresh{Immediate: true, Ref: &ref}, Items: []api.Item{{ID: path, Title: "image.png", Image: &api.Image{Src: durableGridReference(path)}}}}
	return path, ref, view
}

func openDurableGrids(t *testing.T, dir string, snapshots *CommandSnapshots, identity any, revision string) (*DurableGrids, *store.Store) {
	t.Helper()
	ports := real.NewPorts()
	st, err := store.Open(ports.FS, ports.Clock, dir)
	if err != nil {
		t.Fatal(err)
	}
	return NewDurableGrids(st, DurableGridDeps{
		Snapshots: snapshots,
		Identity:  func(string) any { return identity },
		Revision:  func(string, any) (string, bool) { return revision, true },
		Files:     ports.FS,
		OnUI:      func(fn func()) { fn() },
	}), st
}

func TestDurableGridsRestoreTheLastGridAfterALaunch(t *testing.T) {
	dir := t.TempDir()
	path, ref, view := durableGridFixture(t)
	before, after := new(byte), new(byte)

	var first CommandSnapshots
	first.Put(before, ref, view, view.Items)
	grids, st := openDurableGrids(t, dir, &first, before, "rev")
	grids.Remember("ext", before, ref)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	var second CommandSnapshots
	grids, _ = openDurableGrids(t, dir, &second, after, "rev")
	grids.restorePass(t.Context())
	got, ok := second.Get(after, ref)
	if !ok || len(got.Items) != 1 || got.Items[0].ID != path {
		t.Fatalf("the saved grid was not restored: %+v, %v", got.Items, ok)
	}
}

func TestDurableGridsDropAGridFromAnotherInstallation(t *testing.T) {
	dir := t.TempDir()
	_, ref, view := durableGridFixture(t)
	before, after := new(byte), new(byte)

	var first CommandSnapshots
	first.Put(before, ref, view, view.Items)
	grids, st := openDurableGrids(t, dir, &first, before, "old")
	grids.Remember("ext", before, ref)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	var second CommandSnapshots
	grids, _ = openDurableGrids(t, dir, &second, after, "new")
	grids.restorePass(t.Context())
	if _, ok := second.Get(after, ref); ok {
		t.Fatal("a grid saved by another installation was restored")
	}
	doc, err := grids.doc.Load()
	if err != nil || len(doc.Records) != 0 {
		t.Fatalf("the stale record stayed on disk: %+v, %v", doc.Records, err)
	}
}
