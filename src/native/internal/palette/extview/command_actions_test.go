package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestCommandActionUsesCommandInvokerOnlyForInitialRun(t *testing.T) {
	var generic, command []string
	follow := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "follow"}}
	deps := Deps{
		Invoke: func(_ string, in api.Invocation, _ func(api.Result, *api.Failure)) {
			generic = append(generic, in.Ref.ID)
		},
		InvokeCommand: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
			command = append(command, in.Ref.ID)
			if in.Ref.ID == "run" {
				done(api.Result{Next: &follow}, nil)
			}
		},
	}
	action := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "run"}}
	CommandActionFor(deps, "acme", action).Run(&fakeHost{})
	ActionFor(deps, "acme", action).Run(&fakeHost{})
	if len(command) != 1 || command[0] != "run" || len(generic) != 2 || generic[0] != "follow" || generic[1] != "run" {
		t.Fatalf("generic=%v command=%v", generic, command)
	}
}

func TestCommandResultFollowupsAndReturnedViewUseGenericInvoker(t *testing.T) {
	var generic, command []string
	follow := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "follow"}}
	returned := api.View{Kind: api.ViewList, Items: []api.Item{{ID: "row", Title: "Row", Primary: &api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "view"}}}}}
	deps := Deps{
		Invoke: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
			generic = append(generic, in.Ref.ID)
			if in.Ref.ID == "follow" {
				done(api.Result{}, nil)
				return
			}
			if in.Ref.ID == "initial" {
				done(api.Result{Next: &follow}, nil)
				return
			}
			done(api.Result{}, nil)
		},
		InvokeCommand: func(_ string, in api.Invocation, done func(api.Result, *api.Failure)) {
			command = append(command, in.Ref.ID)
			if in.Ref.ID == "initial" {
				done(api.Result{Next: &follow}, nil)
			}
		},
	}
	// First command returns a follow-up action.
	CommandActionFor(deps, "acme", api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "initial"}}).Run(&fakeHost{})
	// A separate command returning a view exposes a regular view action.
	deps.InvokeCommand = func(_ string, _ api.Invocation, done func(api.Result, *api.Failure)) {
		command = append(command, "view-command")
		done(api.Result{View: &returned}, nil)
	}
	host := &fakeHost{}
	CommandActionFor(deps, "acme", api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "view-command"}}).Run(host)
	view := host.pushed[len(host.pushed)-1]
	view.Search("")[0].Actions[0].Run(host)
	if len(generic) != 2 || generic[0] != "follow" || generic[1] != "view" || len(command) != 2 || command[0] != "initial" || command[1] != "view-command" {
		t.Fatalf("generic=%v command=%v", generic, command)
	}
}

func TestCommandConfirmationAndRepeatedBackgroundRuns(t *testing.T) {
	var generic, command int
	var background []bool
	deps := Deps{
		Invoke: func(_ string, in api.Invocation, _ func(api.Result, *api.Failure)) {
			generic++
			background = append(background, in.Background)
		},
		InvokeCommand: func(_ string, in api.Invocation, _ func(api.Result, *api.Failure)) {
			command++
			background = append(background, in.Background)
		},
	}
	action := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "run"}, Background: true, Confirm: &api.Confirmation{}}
	host := &fakeHost{}
	mapped := CommandActionFor(deps, "acme", action)
	mapped.Run(host)
	confirm := host.pushed[0].Search("")[0].Actions[0]
	confirm.Run(host)
	mapped.Run(host)
	confirm = host.pushed[len(host.pushed)-1].Search("")[0].Actions[0]
	confirm.Run(host)
	if command != 2 || generic != 0 || len(background) != 2 || !background[0] || !background[1] {
		t.Fatalf("generic=%d command=%d background=%v", generic, command, background)
	}
}

func TestCommandActionFallsBackToGenericInvoker(t *testing.T) {
	var calls int
	deps := Deps{Invoke: func(string, api.Invocation, func(api.Result, *api.Failure)) { calls++ }}
	CommandActionFor(deps, "acme", api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "run"}}).Run(&fakeHost{})
	if calls != 1 {
		t.Fatalf("generic calls=%d", calls)
	}
}

func TestCommandOnlyInvokerDoesNotReportUnavailable(t *testing.T) {
	var calls int
	CommandActionFor(Deps{InvokeCommand: func(string, api.Invocation, func(api.Result, *api.Failure)) { calls++ }}, "acme", api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "run"}}).Run(&fakeHost{})
	if calls != 1 {
		t.Fatalf("command calls=%d", calls)
	}
}
