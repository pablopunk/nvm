package extview

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestCrossExtensionFollowUpKeepsCommandOrigin(t *testing.T) {
	source, target, replacement := new(int), new(int), new(int)
	current := any(target)
	var calls []api.Invocation
	deps := Deps{
		CommandSnapshotIdentity: func(id string) any {
			if id == "source" {
				return source
			}
			return current
		},
		Invoke: func(id string, in api.Invocation, done func(api.Result, *api.Failure)) {
			if id != "target" {
				t.Fatalf("target=%q", id)
			}
			calls = append(calls, in)
			done(api.Result{}, nil)
		},
		InvokeViewAction: func(id string, in api.Invocation, done func(api.Result, *api.Failure)) {
			if id != "target" {
				t.Fatalf("target=%q", id)
			}
			calls = append(calls, in)
			done(api.Result{}, nil)
		},
	}
	session := NewSession(deps, "source", api.View{})
	host := &fakeHost{}
	follow := api.Action{Kind: api.ActionRun, Ref: &api.ActionRef{ID: "follow"}}
	session.finish(host, "target", "", api.Result{OriginIdentity: target, OriginBound: true, Next: &follow}, nil)
	if len(calls) != 1 || !calls[0].RequireIdentity || calls[0].ExpectedIdentity != target {
		t.Fatalf("calls=%+v", calls)
	}
	follow.Confirm = &api.Confirmation{}
	session.finish(host, "target", "", api.Result{OriginIdentity: target, OriginBound: true, Next: &follow}, nil)
	current = replacement
	host.pushed[len(host.pushed)-1].Search("")[0].Actions[0].Run(host)
	if len(calls) != 2 || !calls[1].RequireIdentity || calls[1].ExpectedIdentity != target {
		t.Fatalf("confirmed calls=%+v", calls)
	}
}

func TestCommandResultChildUsesCapturedIdentity(t *testing.T) {
	source, target, replacement := new(int), new(int), new(int)
	deps := Deps{CommandSnapshotIdentity: func(id string) any {
		if id == "source" {
			return source
		}
		return replacement
	}}
	session := NewSession(deps, "source", api.View{})
	view := api.View{Kind: api.ViewList, Title: "Target"}
	child := session.resultChild("target", api.Result{OriginIdentity: target, OriginBound: true, View: &view})
	if child.installationIdentity != target {
		t.Fatal("child adopted the replacement installation")
	}
}
