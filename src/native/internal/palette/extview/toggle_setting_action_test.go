package extview

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestToggleSettingActionRequiresConfirmationAndUsesOrigin(t *testing.T) {
	identity := new(byte)
	var done func(error)
	calls := 0
	deps := Deps{CommandSnapshotIdentity: func(string) any { return identity }, ToggleSetting: func(id, key string, got any, finish func(error)) {
		calls++
		if id != "acme.settings" || key != "compact" || got != identity {
			t.Fatalf("origin %q key %q identity %v", id, key, got)
		}
		done = finish
	}}
	s := NewSession(deps, "acme.settings", api.ListView("Settings"))
	h := &fakeHost{}
	action := api.Action{Kind: api.ActionToggleSetting, Title: "Toggle", Setting: "compact", Confirm: &api.Confirmation{Title: "Confirm toggle"}}
	s.action(action, "toggle").Run(h)
	if calls != 0 || len(h.pushed) != 1 || h.pushed[0].Title != "Confirm" {
		t.Fatal("confirmation skipped")
	}
	h.pushed[0].Search("")[0].Actions[0].Run(h)
	if calls != 1 {
		t.Fatal("confirmed action did not call service")
	}
	done(nil)
	if h.refresh != 1 || h.hidden {
		t.Fatalf("completion refresh %d hidden %v", h.refresh, h.hidden)
	}
}

func TestToggleSettingActionReportsFailureAndDropsLateResult(t *testing.T) {
	var done func(error)
	deps := Deps{ToggleSetting: func(_ string, _ string, _ any, finish func(error)) { done = finish }}
	s := NewSession(deps, "acme", api.ListView("Settings"))
	h := &fakeHost{}
	action := s.action(api.Action{Kind: api.ActionToggleSetting, Setting: "compact"}, "toggle")
	action.Run(h)
	done(errors.New("denied"))
	if len(h.toasts) != 1 || h.toasts[0] != "denied" || h.refresh != 0 {
		t.Fatalf("failure: %+v", h)
	}
	action.Run(h)
	s.close()
	done(nil)
	if h.refresh != 0 {
		t.Fatal("late result refreshed dismissed view")
	}
}

func TestToggleSettingConfirmedFollowUpPreservesIdentity(t *testing.T) {
	identity := new(byte)
	calls := 0
	deps := Deps{CommandSnapshotIdentity: func(string) any { return identity }, ToggleSetting: func(_ string, _ string, got any, _ func(error)) {
		if got != identity {
			t.Fatal("lost identity")
		}
		calls++
	}}
	s := NewSession(deps, "acme", api.ListView("Settings"))
	h := &fakeHost{}
	a := api.Action{Kind: api.ActionToggleSetting, Setting: "compact", Confirm: &api.Confirmation{Title: "Confirm"}}
	s.finish(h, "acme", "", api.Result{Next: &a}, nil)
	h.pushed[0].Search("")[0].Actions[0].Run(h)
	if calls != 1 {
		t.Fatal("follow-up not dispatched")
	}
}

func TestToggleSettingUsesViewInstallationAfterReplacement(t *testing.T) {
	old, replacement := new(byte), new(byte)
	current := old
	var got any
	deps := Deps{CommandSnapshotIdentity: func(string) any { return current }, ToggleSetting: func(_ string, _ string, identity any, _ func(error)) { got = identity }}
	s := NewSession(deps, "acme", api.ListView("Settings"))
	current = replacement
	s.action(api.Action{Kind: api.ActionToggleSetting, Setting: "compact"}, "toggle").Run(&fakeHost{})
	if got != old {
		t.Fatal("old view used replacement installation authority")
	}
}

func TestToggleSettingInNestedViewKeepsOriginalInstallation(t *testing.T) {
	old, replacement := new(byte), new(byte)
	current := old
	var got any
	deps := Deps{CommandSnapshotIdentity: func(string) any { return current }, ToggleSetting: func(_ string, _ string, identity any, _ func(error)) { got = identity }}
	parent := NewSession(deps, "acme", api.ListView("Parent"))
	host := &fakeHost{}
	current = replacement
	toggle := api.Action{Kind: api.ActionToggleSetting, Setting: "compact"}
	view := api.ListView("Nested", api.Item{ID: "toggle", Title: "Toggle", Primary: api.Primary(toggle)})
	parent.action(api.PushView("Open", view), "open").Run(host)
	host.pushed[0].Search("")[0].Actions[0].Run(host)
	if got != old {
		t.Fatal("nested view acquired replacement installation authority")
	}
}
