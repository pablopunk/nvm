package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestDeclaredDataChangeCoalescesRefresh(t *testing.T) {
	var callbacks []func(api.Result, *api.Failure)
	ref := api.ActionRef{ID: "refresh"}
	s := NewSession(Deps{Invoke: func(id string, in api.Invocation, done func(api.Result, *api.Failure)) {
		if id != "nevermind.menus" || in.Ref.ID != ref.ID {
			t.Fatalf("wrong declared refresh: %s %+v", id, in)
		}
		callbacks = append(callbacks, done)
	}}, "nevermind.menus", api.View{Kind: api.ViewList, Items: []api.Item{{ID: "cached"}}, Refresh: &api.Refresh{Ref: &ref}})
	page := s.Open(&fakeHost{})
	if page.ExtensionID != "nevermind.menus" || page.OnDataChanged == nil {
		t.Fatal("host provenance or declared refresh missing")
	}
	page.OnDataChanged()
	page.OnDataChanged()
	if len(callbacks) != 1 || s.items[0].ID != "cached" {
		t.Fatalf("refresh blocked cached rows or did not coalesce: %d %+v", len(callbacks), s.items)
	}
	callbacks[0](api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: []api.Item{{ID: "obsolete"}}}}, nil)
	if len(callbacks) != 2 || s.items[0].ID != "cached" {
		t.Fatalf("obsolete refresh published: calls=%d items=%v", len(callbacks), s.items)
	}
	callbacks[1](api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: []api.Item{{ID: "fresh"}}}}, nil)
	if len(s.items) != 1 || s.items[0].ID != "fresh" {
		t.Fatalf("trailing refresh missing: %+v", s.items)
	}
	page.OnLeave()
	page.OnDataChanged()
	if len(callbacks) != 2 {
		t.Fatal("dismissed view started a refresh")
	}
}
