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
