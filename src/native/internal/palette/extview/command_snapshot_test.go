package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"testing"
)

func TestCommandSnapshotIsRevisionScopedInertAndReplaced(t *testing.T) {
	c := &CommandSnapshots{}
	identity, other := new(byte), new(byte)
	ref := api.ActionRef{ID: "grid", Args: map[string]string{"scope": "one"}}
	view := api.View{Kind: api.ViewGrid, Title: "Screenshots", Refresh: &api.Refresh{Immediate: true, Ref: &api.ActionRef{ID: "grid"}}}
	items := []api.Item{{ID: "x", Title: "photo", Image: &api.Image{Src: "data:image/png;base64,AAAA", Fallback: "file:///private"}, File: &api.FileRef{Path: "/private"}, Primary: &api.Action{Kind: api.ActionCopyImage, Path: "/private"}}}
	c.Put(identity, ref, view, items)
	got, ok := c.Get(identity, ref)
	if !ok || len(got.Items) != 1 {
		t.Fatal("missing snapshot")
	}
	if got.Items[0].File != nil || got.Items[0].Primary != nil || got.Items[0].Image.Fallback != "" {
		t.Fatalf("privileged fields survived: %+v", got.Items[0])
	}
	got.Items[0].Image.Src = "changed"
	again, _ := c.Get(identity, ref)
	if again.Items[0].Image.Src != "data:image/png;base64,AAAA" {
		t.Fatal("caller mutated cached snapshot")
	}
	if _, ok := c.Get(other, ref); ok {
		t.Fatal("different installation reused snapshot")
	}
	if _, ok := c.Get(identity, api.ActionRef{ID: "grid", Args: map[string]string{"scope": "two"}}); ok {
		t.Fatal("different invocation reused snapshot")
	}
	items[0].Image.Src = "data:image/png;base64,BBBB"
	c.Put(identity, ref, view, items)
	got, _ = c.Get(identity, ref)
	if got.Items[0].Image.Src != "data:image/png;base64,BBBB" {
		t.Fatal("fresh content was not promoted")
	}
	c.Put(identity, ref, view, nil)
	if _, ok := c.Get(identity, ref); ok {
		t.Fatal("empty refresh left stale snapshot")
	}
	view.Actions = []api.Action{{Kind: api.ActionCopyImage, Path: "/private"}}
	c.Put(identity, ref, view, items)
	if _, ok := c.Get(identity, ref); ok {
		t.Fatal("cached a view with unsupported actions")
	}
}
