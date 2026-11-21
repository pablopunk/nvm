package extview

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"testing"
)

func TestShellActionConfirmationFreezesArgumentsAndOrigin(t *testing.T) {
	identity := new(byte)
	current := identity
	calls := 0
	var reply func(api.ShellResult, error)
	deps := Deps{CommandSnapshotIdentity: func(string) any { return current }, ShellAction: func(ctx context.Context, id string, command api.ShellCommand, actor any, done func(api.ShellResult, error)) {
		calls++
		if id != "acme.shell" || actor != identity || command.Command != "tool" || command.Args[0] != "original" || command.Env["MODE"] != "original" || ctx.Err() != nil {
			t.Fatalf("invocation=%s %+v %v", id, command, actor)
		}
		reply = done
	}}
	session := NewSession(deps, "acme.shell", api.ListView("Shell"))
	host := &fakeHost{}
	command := api.ShellCommand{Command: "tool", Args: []string{"original"}, Env: map[string]string{"MODE": "original"}}
	session.action(api.Action{Kind: api.ActionShellExec, Title: "Run tool", Shell: &command}, "shell").Run(host)
	if calls != 0 || len(host.pushed) != 1 || host.pushed[0].Search("")[0].Title != "Run" {
		t.Fatal("default confirmation missing")
	}
	command.Command = "changed"
	command.Args[0] = "changed"
	command.Env["MODE"] = "changed"
	current = new(byte)
	confirm := host.pushed[0].Search("")[0].Actions[0]
	confirm.Run(host)
	confirm.Run(host)
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	reply(api.ShellResult{Stdout: "<script>literal</script>", Stderr: "failed", ExitCode: 7}, nil)
	if len(host.pushed) != 2 || host.pushed[1].Title != "Run tool" || host.hidden || len(host.toasts) != 0 {
		t.Fatal("nonzero result failed to open output in palette")
	}
}

func TestShellActionCannotResumeDismissedConfirmationOrPublishLateResult(t *testing.T) {
	calls := 0
	var reply func(api.ShellResult, error)
	var admitted context.Context
	deps := Deps{ShellAction: func(ctx context.Context, _ string, _ api.ShellCommand, _ any, done func(api.ShellResult, error)) {
		calls++
		reply = done
		admitted = ctx
	}}
	session := NewSession(deps, "acme.shell", api.ListView("Shell"))
	host := &fakeHost{}
	action := api.Action{Kind: api.ActionShellExec, Shell: &api.ShellCommand{Script: "echo fixture"}, Confirm: &api.Confirmation{Title: "Explicit question"}}
	session.action(action, "shell").Run(host)
	question := host.pushed[0]
	question.OnHide()
	question.Search("")[0].Actions[0].Run(host)
	if calls != 0 {
		t.Fatal("dismissed confirmation ran")
	}
	session.action(action, "shell").Run(host)
	host.pushed[1].Search("")[0].Actions[0].Run(host)
	if calls != 1 {
		t.Fatal("explicit confirmation failed")
	}
	session.close()
	if admitted.Err() != context.Canceled {
		t.Fatal("foreground process was not cancelled")
	}
	reply(api.ShellResult{ExitCode: 0}, nil)
	if len(host.pushed) != 2 || len(host.toasts) != 0 {
		t.Fatal("late reply changed dismissed UI")
	}
}

func TestShellActionBackgroundDetachesWithoutOpeningOutput(t *testing.T) {
	var admitted context.Context
	var reply func(api.ShellResult, error)
	session := NewSession(Deps{ShellAction: func(ctx context.Context, _ string, _ api.ShellCommand, _ any, done func(api.ShellResult, error)) {
		admitted = ctx
		reply = done
	}}, "acme.shell", api.ListView("Shell"))
	host := &fakeHost{}
	action := api.Action{Kind: api.ActionShellExec, Shell: &api.ShellCommand{Command: "tool"}, Background: true}
	session.action(action, "shell").Run(host)
	host.pushed[0].Search("")[0].Actions[0].Run(host)
	session.close()
	if admitted.Err() != nil {
		t.Fatal("palette close cancelled detached command")
	}
	reply(api.ShellResult{ExitCode: 0}, nil)
	if len(host.pushed) != 1 {
		t.Fatal("detached result reopened palette")
	}
}

func TestShellActionFollowUpConfirmationAlsoFreezesRequest(t *testing.T) {
	var received api.ShellCommand
	calls := 0
	session := NewSession(Deps{ShellAction: func(_ context.Context, _ string, command api.ShellCommand, _ any, _ func(api.ShellResult, error)) {
		calls++
		received = command
	}}, "acme.shell", api.ListView("Shell"))
	host := &fakeHost{}
	command := api.ShellCommand{Command: "original", Args: []string{"original"}}
	session.runFollowUp(host, "acme.shell", api.Action{Kind: api.ActionShellExec, Shell: &command, Confirm: &api.Confirmation{}}, 0)
	command.Command = "changed"
	command.Args[0] = "changed"
	host.pushed[0].Search("")[0].Actions[0].Run(host)
	if calls != 1 || received.Command != "original" || received.Args[0] != "original" {
		t.Fatalf("follow-up confirmation changed request: %+v", received)
	}
}
