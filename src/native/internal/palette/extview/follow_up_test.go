package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// followUpRig is a session whose invoke port answers each ref from answers and records the refs it was asked for.
type followUpRig struct {
	session *Session
	host    *fakeHost
	asked   []string
	answers map[string]api.Result
}

func newFollowUpRig(t *testing.T, answers map[string]api.Result) *followUpRig {
	t.Helper()
	r := &followUpRig{host: &fakeHost{}, answers: answers}
	deps := Deps{
		Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
			r.asked = append(r.asked, in.Ref.ID)
			done(r.answers[in.Ref.ID], nil)
		},
		Files: fake.NewFS(),
	}
	r.session = NewSession(deps, "acme.follow", api.View{Kind: api.ViewList, Title: "Follow"})
	return r
}

func runRef(id string) *api.Action {
	return &api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: id}}
}

func TestFollowUpRunsItsActionAndAppliesItsResult(t *testing.T) {
	list := api.ListView("Next list")
	r := newFollowUpRig(t, map[string]api.Result{"list": {View: &list}})

	r.session.finish(r.host, "acme.follow", "", api.Result{Next: runRef("list")}, nil)

	if len(r.asked) != 1 || r.asked[0] != "list" {
		t.Fatalf("asked = %v; want the follow-up action once", r.asked)
	}
	if len(r.host.pushed) != 1 || r.host.pushed[0].Title != "Next list" {
		t.Fatalf("pushed = %d views; want the follow-up's view", len(r.host.pushed))
	}
}

func TestFollowUpTakesPrecedenceOverTheResultsOwnView(t *testing.T) {
	view := api.ListView("Ignored")
	r := newFollowUpRig(t, nil)
	pop := api.Action{Kind: api.ActionPop}

	r.session.finish(r.host, "acme.follow", "", api.Result{Next: &pop, View: &view}, nil)

	if r.host.pops != 1 || len(r.host.pushed) != 0 {
		t.Fatalf("pops = %d, pushed = %d; want the pop only, as Electron runs the action instead of the view", r.host.pops, len(r.host.pushed))
	}
}

func TestFollowUpDeclarativeActionRunsOnTheHost(t *testing.T) {
	r := newFollowUpRig(t, nil)
	r.session.finish(r.host, "acme.follow", "", api.Result{Next: &api.Action{Kind: api.ActionPop}}, nil)
	if r.host.pops != 1 || len(r.asked) != 0 {
		t.Fatalf("pops = %d, asked = %v; want a host pop and no extension call", r.host.pops, r.asked)
	}
}

func TestFollowUpWithConfirmationAsksFirst(t *testing.T) {
	r := newFollowUpRig(t, nil)
	action := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "delete"}, Confirm: &api.Confirmation{Title: "Delete?", ConfirmLabel: "Delete"}}

	r.session.finish(r.host, "acme.follow", "", api.Result{Next: &action}, nil)

	if len(r.asked) != 0 || len(r.host.pushed) != 1 || r.host.pushed[0].Title != "Confirm" {
		t.Fatalf("asked = %v, pushed = %d; want the confirmation page and no run", r.asked, len(r.host.pushed))
	}
}

func TestFollowUpChainIsBounded(t *testing.T) {
	r := newFollowUpRig(t, map[string]api.Result{"loop": {Next: runRef("loop")}})

	r.session.finish(r.host, "acme.follow", "", api.Result{Next: runRef("loop")}, nil)

	if len(r.asked) != maxFollowUps || len(r.host.toasts) != 1 {
		t.Fatalf("asked %d times, toasts = %v; want %d runs and one notice", len(r.asked), r.host.toasts, maxFollowUps)
	}
}

func TestFailedResultIgnoresItsFollowUp(t *testing.T) {
	r := newFollowUpRig(t, nil)
	r.session.finish(r.host, "acme.follow", "", api.Result{Next: runRef("list")}, &api.Failure{Code: api.CodeInternal, Message: "boom"})
	if len(r.asked) != 0 || len(r.host.toasts) != 1 || r.host.toasts[0] != "boom" {
		t.Fatalf("asked = %v, toasts = %v; want the failure shown and no follow-up", r.asked, r.host.toasts)
	}
}
