package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"testing"
)

func TestSystemConfirmationIsSingleUseAndKeepsOrigin(t *testing.T) {
	identity := new(byte)
	current := identity
	calls := 0
	deps := Deps{CommandSnapshotIdentity: func(string) any { return current }, SystemCommand: func(id, op, pane string, got any, done func(string)) {
		calls++
		if id != "acme.system" || op != "empty-trash" || pane != "" || got != identity {
			t.Fatal("confirmation changed its originating invocation")
		}
		done("")
	}}
	session := NewSession(deps, "acme.system", api.ListView("System"))
	host := &fakeHost{}
	action := api.SystemOp("Empty Trash", "empty-trash")
	action.Confirm = &api.Confirmation{}
	session.action(action, "trash").Run(host)
	if calls != 0 || len(host.pushed) != 1 {
		t.Fatal("confirmation skipped")
	}
	current = new(byte)
	confirm := host.pushed[0].Search("")[0].Actions[0]
	confirm.Run(host)
	confirm.Run(host)
	if calls != 1 {
		t.Fatal("affirmative continuation dispatched more than once")
	}
}

func TestSystemConfirmationCannotResumeAfterHide(t *testing.T) {
	calls := 0
	session := NewSession(Deps{SystemCommand: func(string, string, string, any, func(string)) { calls++ }}, "acme.system", api.ListView("System"))
	host := &fakeHost{}
	action := api.SystemOp("Empty Trash", "empty-trash")
	action.Confirm = &api.Confirmation{}
	session.action(action, "trash").Run(host)
	page := host.pushed[0]
	confirm := page.Search("")[0].Actions[0]
	page.OnHide()
	confirm.Run(host)
	if calls != 0 {
		t.Fatal("dismissed question dispatched")
	}
}
