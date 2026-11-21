package extview

import (
	"context"
	"crypto/sha256"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"testing"
)

func TestCachedCommandRunsAndHonorsChangedNavigation(t *testing.T) {
	cache := &CommandSnapshots{}
	id := new(byte)
	ref := api.ActionRef{ID: "grid"}
	v := api.View{Kind: api.ViewGrid, Title: "Private", Refresh: &api.Refresh{Immediate: true, Ref: &ref}}
	cache.Put(id, ref, v, []api.Item{{ID: "a", Title: "Secret", Image: &api.Image{Src: "data:image/png;base64,AAAA"}}})
	var finish func(api.Result, *api.Failure)
	var called int
	deps := Deps{CommandSnapshots: cache, CommandSnapshotIdentity: func(string) any { return id }, InvokeCommand: func(string, api.Invocation, func(api.Result, *api.Failure)) { t.Fatal("context command not used") }, InvokeCommandContext: func(_ context.Context, _ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		called++
		if !in.ProtectSnapshot {
			t.Fatal("missing protection")
		}
		finish = done
	}}
	h := &fakeHost{}
	CommandActionFor(deps, "x", api.Action{Kind: api.ActionRun, Ref: &ref}).Run(h)
	if called != 1 || len(h.pushed) != 1 {
		t.Fatal("command not dispatched with snapshot")
	}
	rows := h.pushed[0].Search("")
	if len(rows) != 1 || len(rows[0].Actions) != 0 {
		t.Fatal("cached rows are actionable")
	}
	finish(api.Result{Navigation: api.NavReplace, View: &api.View{Kind: api.ViewList, Title: "Changed"}}, nil)
	if h.pops != 2 || len(h.pushed) != 2 || h.pushed[1].Title != "Changed" {
		t.Fatalf("navigation lost: %+v", h)
	}
	if _, ok := cache.Get(id, ref); ok {
		t.Fatal("old snapshot remained eligible")
	}
}

func TestPermissionFailureErasesSnapshotAndVisibleContent(t *testing.T) {
	cache := &CommandSnapshots{}
	id := new(byte)
	ref := api.ActionRef{ID: "grid"}
	v := api.View{Kind: api.ViewGrid, Title: "Private", Placeholder: "Private search", Hint: "Private hint", Items: []api.Item{{ID: "secret", Title: "Secret"}}, Refresh: &api.Refresh{Ref: &ref, Immediate: true}}
	cache.Put(id, ref, v, v.Items)
	var done func(api.Result, *api.Failure)
	s := NewSession(Deps{CommandSnapshots: cache, CommandSnapshotIdentity: func(string) any { return id }, Invoke: func(_ string, _ api.Invocation, cb func(api.Result, *api.Failure)) { done = cb }}, "x", v)
	s.snapshotRef = &ref
	s.snapshotSourcePaths = map[[32]byte]bool{sha256.Sum256([]byte("/private.png")): true}
	s.snapshotIdentity = id
	h := &fakeHost{}
	page := s.Open(h)
	token := s.issue("secret", route{extID: "x", action: api.Action{Kind: api.ActionRun, Ref: &ref}})
	done(api.Result{SnapshotEvidence: &api.SnapshotEvidence{FilesAuthorityDenied: true}}, &api.Failure{Code: api.CodePermission, Message: "denied"})
	if _, ok := cache.Get(id, ref); ok {
		t.Fatal("denied cache remained")
	}
	if page.Title != "Access unavailable" || page.Placeholder != "" || page.Hint != "" || len(s.items) != 0 || len(s.view.Items) != 0 {
		t.Fatal("private content remained visible")
	}
	s.runHandle(h, token)
	if len(h.toasts) == 0 || h.toasts[len(h.toasts)-1] != "This action expired; reopen the view" {
		t.Fatal("denied action remained live")
	}
	var protected bool
	deps := Deps{CommandSnapshots: cache, CommandSnapshotIdentity: func(string) any { return id }, InvokeCommand: func(_ string, in api.Invocation, _ func(api.Result, *api.Failure)) { protected = in.ProtectSnapshot }}
	CommandActionFor(deps, "x", api.Action{Kind: api.ActionRun, Ref: &ref}).Run(&fakeHost{})
	if !protected {
		t.Fatal("reopen lost denial monitoring")
	}
}

func TestObsoleteDenialCannotEraseNewerSnapshot(t *testing.T) {
	cache := &CommandSnapshots{}
	id := new(byte)
	ref := api.ActionRef{ID: "grid"}
	v := api.View{Kind: api.ViewGrid, Title: "x", Refresh: &api.Refresh{Immediate: true, Ref: &ref}}
	cache.Put(id, ref, v, []api.Item{{ID: "old", Title: "old"}})
	old := cache.Version(id, ref)
	cache.Put(id, ref, v, []api.Item{{ID: "new", Title: "new"}})
	cache.RemoveVersion(id, ref, old)
	got, ok := cache.Get(id, ref)
	if !ok || len(got.Items) != 1 || got.Items[0].ID != "new" {
		t.Fatal("obsolete denial removed newer snapshot")
	}
}
