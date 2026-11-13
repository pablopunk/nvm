package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"testing"
)

func TestRefreshReinvokesCommandAndAppliesReturnedGrid(t *testing.T) {
	var invocation api.Invocation
	var finish func(api.Result, *api.Failure)
	s := NewSession(Deps{Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) { invocation = in; finish = done }}, "test", api.View{Kind: api.ViewGrid, Items: []api.Item{{ID: "cached"}}, Refresh: &api.Refresh{Mode: api.PatchReplace, Ref: &api.ActionRef{ID: "grid"}}})
	s.loading = true
	s.refreshNow()
	if !invocation.Launch.Refresh || invocation.Launch.Reason != "refresh" || invocation.Ref.ID != "grid" {
		t.Fatalf("refresh invocation %+v", invocation)
	}
	if len(s.items) != 1 || s.items[0].ID != "cached" {
		t.Fatal("cached item removed during refresh")
	}
	finish(api.Result{View: &api.View{Kind: api.ViewGrid, Items: []api.Item{{ID: "fresh"}}}}, nil)
	if s.loading || len(s.items) != 1 || s.items[0].ID != "fresh" {
		t.Fatalf("refresh loading=%v items=%+v", s.loading, s.items)
	}
}
func TestDismissedRefreshCannotReplaceRows(t *testing.T) {
	var finish func(api.Result, *api.Failure)
	s := NewSession(Deps{Invoke: func(_ string, _ api.Invocation, done func(api.Result, *api.Failure)) { finish = done }}, "test", api.View{Kind: api.ViewGrid, Items: []api.Item{{ID: "cached"}}, Refresh: &api.Refresh{Ref: &api.ActionRef{ID: "grid"}}})
	s.refreshNow()
	s.close()
	finish(api.Result{View: &api.View{Kind: api.ViewGrid, Items: []api.Item{{ID: "late"}}}}, nil)
	if len(s.items) != 1 || s.items[0].ID != "cached" {
		t.Fatalf("dismissed refresh changed rows %+v", s.items)
	}
}
