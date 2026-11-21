package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/palette"
)

func TestLateAutosaveRevalidatesOriginAndCoalesces(t *testing.T) {
	var save func(api.Result, *api.Failure)
	var refreshes []func(api.Result, *api.Failure)
	deps := Deps{Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		switch in.Ref.ID {
		case "save":
			save = done
		case "refresh":
			refreshes = append(refreshes, done)
		}
	}}
	ref := api.ActionRef{ID: "refresh"}
	parent := NewSession(deps, "notes", api.View{Kind: api.ViewList, Items: []api.Item{{ID: "cached"}}, Refresh: &api.Refresh{Ref: &ref}})
	parent.Open(&fakeHost{})
	child := NewSession(deps, "notes", api.View{})
	child.parent = parent
	ed := newEditorState(child, "note", "new text", "", false)
	ed.draft = &draftTrack{options: api.DraftOptions{Autosave: &api.Action{Ref: &api.ActionRef{ID: "save"}}}}
	ed.autosave()
	parent.refreshNow() // Back reveals the list before the save finishes.
	save(api.Result{DataChanged: true}, nil)
	if len(refreshes) != 1 || parent.items[0].ID != "cached" {
		t.Fatal("cached rows were lost or refresh was not coalesced")
	}
	refreshes[0](api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: []api.Item{{ID: "stale"}}}}, nil)
	if len(refreshes) != 2 || parent.items[0].ID != "cached" {
		t.Fatal("obsolete response was published")
	}
	refreshes[1](api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: []api.Item{{ID: "saved"}}}}, nil)
	if parent.items[0].ID != "saved" {
		t.Fatal("completed save was not reflected")
	}
}

func TestAutosaveFailureAndClosedOriginDoNotRefresh(t *testing.T) {
	var save func(api.Result, *api.Failure)
	calls := 0
	deps := Deps{Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		if in.Ref.ID == "save" {
			save = done
		} else {
			calls++
		}
	}}
	ref := api.ActionRef{ID: "refresh"}
	parent := NewSession(deps, "notes", api.View{Kind: api.ViewList, Refresh: &api.Refresh{Ref: &ref}})
	parent.Open(&fakeHost{})
	child := NewSession(deps, "notes", api.View{})
	child.parent = parent
	ed := newEditorState(child, "note", "text", "", false)
	ed.draft = &draftTrack{options: api.DraftOptions{Autosave: &api.Action{Ref: &api.ActionRef{ID: "save"}}}}
	ed.autosave()
	save(api.Result{}, &api.Failure{Code: api.CodeCancelled, Message: "cancelled"})
	ed.autosave()
	parent.close()
	save(api.Result{DataChanged: true}, nil)
	if calls != 0 {
		t.Fatal("failed save or closed origin refreshed")
	}
}

type cancelledActionHost struct{ pageHost }

func (h *cancelledActionHost) BeginAction() func() bool { return func() bool { return false } }

func TestCompletedCreateRefreshesOriginAfterNavigationCancellation(t *testing.T) {
	var complete func(api.Result, *api.Failure)
	refreshes := 0
	deps := Deps{Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		if in.Ref.ID == "create" {
			complete = done
		} else {
			refreshes++
		}
	}}
	ref := api.ActionRef{ID: "refresh"}
	s := NewSession(deps, "notes", api.View{Kind: api.ViewList, Refresh: &api.Refresh{Ref: &ref}})
	h := &cancelledActionHost{}
	s.Open(h)
	s.invoke(h, route{extID: "notes", action: api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "create"}}})
	complete(api.Result{DataChanged: true, View: &api.View{Kind: api.ViewEditor}}, nil)
	if refreshes != 1 || len(h.pushed) != 0 {
		t.Fatalf("refreshes=%d pushed=%d", refreshes, len(h.pushed))
	}
}

type targetedRefreshHost struct {
	pageHost
	visible *palette.View
}

func (h *targetedRefreshHost) RefreshTop(pred func(*palette.View) bool) {
	if pred(h.visible) {
		h.refreshes++
	}
}

func TestReplacedEditorLateAutosaveRefreshesSurvivingList(t *testing.T) {
	var save func(api.Result, *api.Failure)
	var refresh func(api.Result, *api.Failure)
	deps := Deps{Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		if in.Ref.ID == "save" {
			save = done
		} else {
			refresh = done
		}
	}}
	ref := api.ActionRef{ID: "refresh"}
	list := NewSession(deps, "notes", api.View{Kind: api.ViewList, Items: []api.Item{{ID: "old"}}, Refresh: &api.Refresh{Ref: &ref}})
	h := &targetedRefreshHost{}
	listPage := list.Open(h)
	original := list.navigationChild("notes", api.View{Kind: api.ViewEditor}, api.NavPush)
	replacement := original.navigationChild("notes", api.View{Kind: api.ViewEditor}, api.NavReplace)
	ed := newEditorState(replacement, "note", "saved", "", false)
	ed.draft = &draftTrack{options: api.DraftOptions{Autosave: &api.Action{Ref: &api.ActionRef{ID: "save"}}}}
	ed.autosave()
	h.visible = &palette.View{Title: "Another page"}
	save(api.Result{DataChanged: true}, nil)
	if refresh == nil {
		t.Fatal("surviving list was not revalidated")
	}
	refresh(api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: []api.Item{{ID: "saved"}}}}, nil)
	if list.items[0].ID != "saved" || h.refreshes != 0 {
		t.Fatal("background refresh disturbed visible page or missed saved data")
	}
	h.visible = listPage
	list.refreshHost()
	if h.refreshes != 1 {
		t.Fatal("visible list did not redraw")
	}
}

func TestDelayedDeleteRevalidatesAfterNavigation(t *testing.T) {
	var deletion func(api.Result, *api.Failure)
	var refreshes []func(api.Result, *api.Failure)
	deps := Deps{Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		if in.Ref.ID == "delete" {
			deletion = done
		} else {
			refreshes = append(refreshes, done)
		}
	}}
	ref := api.ActionRef{ID: "refresh"}
	s := NewSession(deps, "notes", api.View{Kind: api.ViewList, Items: []api.Item{{ID: "old"}}, Refresh: &api.Refresh{Ref: &ref}})
	h := &cancelledActionHost{}
	s.Open(h)
	s.invoke(h, route{extID: "notes", action: api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "delete"}}})
	s.refreshNow()
	deletion(api.Result{DataChanged: true, Patch: &api.ViewPatch{Mode: api.PatchReplace}}, nil)
	refreshes[0](api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace, Items: []api.Item{{ID: "old"}}}}, nil)
	if len(refreshes) != 2 || len(s.items) != 1 {
		t.Fatal("stale refresh was published or trailing refresh missing")
	}
	refreshes[1](api.Result{Patch: &api.ViewPatch{Mode: api.PatchReplace}}, nil)
	if len(s.items) != 0 {
		t.Fatal("deleted row remains")
	}
}

func TestPartialAutosaveFailureRefreshesParentAndReportsError(t *testing.T) {
	var save func(api.Result, *api.Failure)
	refreshes := 0
	deps := Deps{Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
		if in.Ref.ID == "save" {
			save = done
		} else {
			refreshes++
			done(api.Result{}, nil)
		}
	}}
	ref := api.ActionRef{ID: "refresh"}
	parent := NewSession(deps, "notes", api.View{Kind: api.ViewList, Refresh: &api.Refresh{Ref: &ref}})
	editor := NewSession(deps, "notes", api.View{})
	editor.parent = parent
	ed := &editorState{sess: editor, draft: &draftTrack{options: api.DraftOptions{Autosave: &api.Action{Ref: &api.ActionRef{ID: "save"}}}}}
	ed.autosave()
	save(api.Result{DataChanged: true}, &api.Failure{Message: "commit failed"})
	if refreshes != 1 || ed.status != "commit failed" {
		t.Fatalf("refreshes=%d status=%q", refreshes, ed.status)
	}
}
