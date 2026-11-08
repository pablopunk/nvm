package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestConfirmedFollowUpChainStopsAtLimit(t *testing.T) {
	identity := new(int)
	calls := 0
	next := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "loop"}, Confirm: &api.Confirmation{}}
	deps := Deps{
		CommandSnapshotIdentity: func(string) any { return identity },
		InvokeViewAction: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
			if !in.RequireIdentity || in.ExpectedIdentity != identity {
				t.Fatalf("lost origin: %+v", in)
			}
			calls++
			done(api.Result{Next: &next, OriginIdentity: identity, OriginBound: true}, nil)
		},
	}
	s := NewSession(deps, "test", api.View{})
	h := &fakeHost{}
	s.finish(h, "test", "", api.Result{Next: &next, OriginIdentity: identity, OriginBound: true}, nil)
	for i := 0; i < maxFollowUps; i++ {
		if len(h.pushed) != i+1 {
			t.Fatalf("confirmation pages=%d at %d", len(h.pushed), i)
		}
		h.pushed[i].Search("")[0].Actions[0].Run(h)
	}
	if calls != maxFollowUps || len(h.pushed) != maxFollowUps || len(h.toasts) == 0 || h.toasts[len(h.toasts)-1] != "This action asked for too many follow-up actions" {
		t.Fatalf("calls=%d pages=%d toasts=%v", calls, len(h.pushed), h.toasts)
	}
}

func TestCrossExtensionDeclarativeNavigationKeepsOrigin(t *testing.T) {
	source, target, replacement := new(int), new(int), new(int)
	current := any(target)
	var invoked api.Invocation
	deps := Deps{InvokeViewAction: func(_ string, in api.Invocation, _ func(api.Result, *api.Failure)) { invoked = in }, CommandSnapshotIdentity: func(id string) any {
		if id == "source" {
			return source
		}
		return current
	}}
	s := NewSession(deps, "source", api.View{})
	h := &fakeHost{}
	view := api.ListView("Target", api.Item{ID: "row", Title: "Row", Primary: ptrAction(api.Run("open", "Open", nil))})
	next := api.Action{Kind: api.ActionPushView, View: &view}
	s.finish(h, "target", "", api.Result{Next: &next, OriginIdentity: target, OriginBound: true}, nil)
	if len(h.pushed) != 1 || h.pushed[0].ExtensionID != "target" {
		t.Fatalf("pages=%v", h.pushed)
	}
	current = replacement
	rows := h.pushed[0].Search("")
	if len(rows) != 1 || len(rows[0].Actions) == 0 {
		t.Fatalf("child rows=%v", rows)
	}
	rows[0].Actions[0].Run(h)
	if !invoked.RequireIdentity || invoked.ExpectedIdentity != target {
		t.Fatalf("child action origin=%+v", invoked)
	}
	s.finish(h, "target", "", api.Result{Next: &next, OriginIdentity: target, OriginBound: true}, nil)
	if len(h.pushed) != 1 || len(h.toasts) == 0 {
		t.Fatalf("stale navigation: pages=%d toasts=%v", len(h.pushed), h.toasts)
	}
}

func TestCrossExtensionDeclarativeWindowUsesTarget(t *testing.T) {
	source, target := new(int), new(int)
	var got string
	deps := Deps{
		CommandSnapshotIdentity: func(id string) any {
			if id == "source" {
				return source
			}
			return target
		},
		Window: func(_ search.Host, id string, _ api.Action) { got = id },
	}
	s := NewSession(deps, "source", api.View{})
	h := &fakeHost{}
	action := api.Action{Kind: api.ActionWindow}
	s.finish(h, "target", "", api.Result{Next: &action, OriginIdentity: target, OriginBound: true}, nil)
	if got != "target" {
		t.Fatalf("window origin=%q", got)
	}
}

func TestHeldFollowUpDoesNotNavigateAfterDismissalButInvalidates(t *testing.T) {
	identity := new(int)
	cache := NewMemoryCache()
	lease := cache.acquire("target", identity)
	before := lease.version()
	var done func(api.Result, *api.Failure)
	deps := Deps{
		Cache:                   cache,
		CommandSnapshotIdentity: func(string) any { return identity },
		Invoke:                  func(_ string, _ api.Invocation, cb func(api.Result, *api.Failure)) { done = cb },
	}
	session := NewSession(deps, "source", api.View{})
	host := &fakeHost{}
	action := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "finish"}}
	session.finish(host, "target", "", api.Result{Next: &action, OriginIdentity: identity, OriginBound: true}, nil)
	if done == nil {
		t.Fatal("follow-up was not started")
	}
	session.close()
	view := api.ListView("late")
	done(api.Result{DataChanged: true, View: &view, OriginIdentity: identity, OriginBound: true}, nil)
	if len(host.pushed) != 0 || lease.version() == before {
		t.Fatalf("pages=%d epoch=%d", len(host.pushed), lease.version())
	}
	lease.release()
}

func TestHeldFollowUpDropsResultAfterTargetReplacement(t *testing.T) {
	identity, replacement := new(int), new(int)
	current := any(identity)
	var done func(api.Result, *api.Failure)
	deps := Deps{
		CommandSnapshotIdentity: func(id string) any {
			if id == "source" {
				return nil
			}
			return current
		},
		Invoke: func(_ string, _ api.Invocation, cb func(api.Result, *api.Failure)) { done = cb },
	}
	session := NewSession(deps, "source", api.View{})
	host := &fakeHost{}
	action := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "finish"}}
	session.finish(host, "target", "", api.Result{Next: &action, OriginIdentity: identity, OriginBound: true}, nil)
	current = replacement
	view := api.ListView("late")
	done(api.Result{View: &view, OriginIdentity: identity, OriginBound: true}, nil)
	if len(host.pushed) != 0 {
		t.Fatal("old installation navigated after replacement")
	}
}

func TestCrossExtensionAsyncDeclarativeUsesRealPageLifetime(t *testing.T) {
	for _, dismissed := range []bool{false, true} {
		t.Run(map[bool]string{false: "open", true: "dismissed"}[dismissed], func(t *testing.T) {
			var done func(api.Result, *api.Failure)
			deps := Deps{
				Chats:  &recordingChats{},
				Invoke: func(_ string, _ api.Invocation, cb func(api.Result, *api.Failure)) { done = cb },
			}
			session := NewSession(deps, "source", api.View{})
			host := &fakeHost{}
			action := api.Action{Kind: api.ActionBuilder, Op: api.BuilderList, Builder: &api.BuilderAction{}}
			session.finish(host, "nevermind.builder", "", api.Result{Next: &action, OriginBound: true}, nil)
			if done == nil {
				t.Fatal("declarative action did not start")
			}
			if dismissed {
				session.close()
			}
			view := api.ListView("Completed")
			done(api.Result{View: &view, OriginBound: true}, nil)
			if (len(host.pushed) == 1) == dismissed {
				t.Fatalf("dismissed=%t pages=%d", dismissed, len(host.pushed))
			}
		})
	}
}

func TestCrossExtensionAsyncDeclarativeRejectsReplacement(t *testing.T) {
	first, replacement := new(int), new(int)
	current := any(first)
	var done func(api.Result, *api.Failure)
	deps := Deps{
		Chats: &recordingChats{},
		CommandSnapshotIdentity: func(id string) any {
			if id == "source" {
				return nil
			}
			return current
		},
		Invoke: func(_ string, _ api.Invocation, cb func(api.Result, *api.Failure)) { done = cb },
	}
	session := NewSession(deps, "source", api.View{})
	host := &fakeHost{}
	action := api.Action{Kind: api.ActionBuilder, Op: api.BuilderList, Builder: &api.BuilderAction{}}
	session.finish(host, "nevermind.builder", "", api.Result{Next: &action, OriginIdentity: first, OriginBound: true}, nil)
	if done == nil {
		t.Fatal("action not started")
	}
	current = replacement
	view := api.ListView("stale")
	done(api.Result{View: &view}, nil)
	if len(host.pushed) != 0 {
		t.Fatal("stale result navigated")
	}
}

func TestDeclarativePasteCarriesTargetIdentity(t *testing.T) {
	source, target := new(int), new(int)
	var gotID string
	var gotIdentity any
	deps := Deps{
		CommandSnapshotIdentity: func(id string) any {
			if id == "source" {
				return source
			}
			return target
		},
		PasteBound: func(_ search.Host, id string, identity any, _ api.Action) { gotID, gotIdentity = id, identity },
	}
	session := NewSession(deps, "source", api.View{})
	host := &fakeHost{}
	action := api.PasteText("Paste", "fixture")
	session.finish(host, "target", "", api.Result{Next: &action, OriginIdentity: target, OriginBound: true}, nil)
	if gotID != "target" || gotIdentity != target {
		t.Fatalf("paste id=%q identity=%v", gotID, gotIdentity)
	}
}

func TestHeldFollowUpNavigationGuardKeepsMutation(t *testing.T) {
	identity := new(int)
	cache := NewMemoryCache()
	lease := cache.acquire("target", identity)
	before := lease.version()
	var done func(api.Result, *api.Failure)
	deps := Deps{
		Cache:                   cache,
		CommandSnapshotIdentity: func(string) any { return identity },
		Invoke:                  func(_ string, _ api.Invocation, cb func(api.Result, *api.Failure)) { done = cb },
	}
	session := NewSession(deps, "source", api.View{})
	host := &cancelledActionHost{}
	action := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "run"}}
	session.finish(host, "target", "", api.Result{Next: &action, OriginIdentity: identity, OriginBound: true}, nil)
	if done == nil {
		t.Fatal("follow-up not dispatched")
	}
	view := api.ListView("late")
	done(api.Result{DataChanged: true, View: &view, OriginIdentity: identity, OriginBound: true}, nil)
	if len(host.pushed) != 0 || lease.version() == before {
		t.Fatal("obsolete result navigated or lost mutation")
	}
	lease.release()
}

func TestCrossExtensionWindowCarriesCapturedIdentity(t *testing.T) {
	source, target := new(int), new(int)
	var gotID string
	var gotIdentity any
	deps := Deps{
		CommandSnapshotIdentity: func(id string) any {
			if id == "source" {
				return source
			}
			return target
		},
		WindowBound: func(id string, identity any, _ api.Action) { gotID, gotIdentity = id, identity },
	}
	session := NewSession(deps, "source", api.View{})
	host := &fakeHost{}
	action := api.Action{Kind: api.ActionWindow}
	session.finish(host, "target", "", api.Result{Next: &action, OriginIdentity: target, OriginBound: true}, nil)
	if gotID != "target" || gotIdentity != target {
		t.Fatalf("window id=%q identity=%v", gotID, gotIdentity)
	}
}
