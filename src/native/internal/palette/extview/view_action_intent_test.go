package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"testing"
)

func TestViewActionIntentConfirmationAndFollowUp(t *testing.T) {
	var generic, view, command int
	follow := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "follow"}}
	deps := Deps{
		Invoke: func(_ string, _ api.Invocation, _ func(api.Result, *api.Failure)) { generic++ },
		InvokeViewAction: func(_ string, _ api.Invocation, done func(api.Result, *api.Failure)) {
			view++
			done(api.Result{Next: &follow}, nil)
		},
		InvokeCommand: func(_ string, _ api.Invocation, _ func(api.Result, *api.Failure)) { command++ },
	}
	action := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "private"}, Confirm: &api.Confirmation{}}
	host := &fakeHost{}
	s := NewSession(deps, "acme", api.View{})
	mapped := s.action(action, "row")
	mapped.Run(host)
	if view != 0 || generic != 0 {
		t.Fatal("invoked before confirmation")
	}
	host.pushed[0].Search("")[1].Actions[0].Run(host)
	if view != 0 {
		t.Fatal("cancel invoked action")
	}
	mapped.Run(host)
	host.pushed[len(host.pushed)-1].Search("")[0].Actions[0].Run(host)
	if view != 1 || generic != 1 {
		t.Fatalf("view=%d generic=%d", view, generic)
	}
	action.Confirm = nil
	ActionFor(deps, "acme", action).Run(host)
	CommandActionFor(deps, "acme", action).Run(host)
	if generic != 2 || command != 1 || view != 1 {
		t.Fatalf("generic=%d command=%d view=%d", generic, command, view)
	}
}

func TestViewInvocationRetainsIdentityThroughConfirmationAndFollowUp(t *testing.T) {
	first, replacement := new(int), new(int)
	current := any(first)
	var calls []api.Invocation
	follow := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "follow"}, Confirm: &api.Confirmation{}}
	deps := Deps{
		CommandSnapshotIdentity: func(string) any { return current },
		InvokeViewAction: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
			calls = append(calls, in)
			if in.Ref.ID == "initial" {
				done(api.Result{Next: &follow}, nil)
			}
		},
	}
	session := NewSession(deps, "acme", api.View{})
	host := &fakeHost{}
	action := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "initial"}, Confirm: &api.Confirmation{}}
	session.action(action, "row").Run(host)
	host.pushed[0].Search("")[0].Actions[0].Run(host)
	current = replacement
	if len(calls) != 1 || !calls[0].RequireIdentity || calls[0].ExpectedIdentity != first {
		t.Fatalf("initial calls=%+v", calls)
	}
	host.pushed[len(host.pushed)-1].Search("")[0].Actions[0].Run(host)
	if len(calls) != 2 || !calls[1].RequireIdentity || calls[1].ExpectedIdentity != first {
		t.Fatalf("follow-up calls=%+v", calls)
	}
}

func TestConfirmationKeepsOriginWhenInstallationChanges(t *testing.T) {
	first, replacement := new(int), new(int)
	current := any(first)
	var received api.Invocation
	deps := Deps{
		CommandSnapshotIdentity: func(string) any { return current },
		InvokeViewAction:        func(_ string, in api.Invocation, _ func(api.Result, *api.Failure)) { received = in },
	}
	session := NewSession(deps, "acme", api.View{})
	host := &fakeHost{}
	session.action(api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "delete"}, Confirm: &api.Confirmation{}}, "row").Run(host)
	current = replacement
	host.pushed[0].Search("")[0].Actions[0].Run(host)
	if !received.RequireIdentity || received.ExpectedIdentity != first {
		t.Fatalf("confirmation lost origin: %+v", received)
	}
}
