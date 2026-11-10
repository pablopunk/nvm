package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
	"testing"
)

func TestOperationRequirementsFilterRowsWithoutErasingSnapshots(t *testing.T) {
	supported := false
	deps := Deps{OperationsSupported: func(required []api.Operation) bool { return len(required) == 0 || supported }}
	primary := api.Run("switch", "Switch", nil)
	required := api.Item{ID: "switch", Title: "Switch", Requires: []api.Operation{api.OperationSwitchDesktop}, Primary: &primary}
	public := api.Item{ID: "public", Title: "Public"}
	session := NewSession(deps, "acme.test", api.ListView("Test", required, public))
	rows := session.appendSectionRows(nil, "", session.items, "")
	if len(rows) != 1 || rows[0].Title != "Public" || len(session.items) != 2 {
		t.Fatal("required row filtering changed source or public content")
	}
	supported = true
	rows = session.appendSectionRows(nil, "", session.items, "")
	if len(rows) != 2 {
		t.Fatal("source could not be projected again")
	}
	if len(required.Primary.Requires) != 0 {
		t.Fatal("source action was modified")
	}
}

func TestOperationRequirementsRecheckCapturedActionsAndNeverPromoteSecondary(t *testing.T) {
	supported := true
	effects := 0
	deps := Deps{OperationsSupported: func(required []api.Operation) bool { return len(required) == 0 || supported }, CopyText: func(search.Host, string) { effects++ }}
	primary := api.CopyText("Copy", "hello", false)
	primary.Requires = []api.Operation{api.OperationFilePreview}
	action := ActionFor(deps, "acme.test", primary)
	supported = false
	host := &pageHost{}
	action.Run(host)
	if effects != 0 || len(host.toasts) != 1 {
		t.Fatal("captured action bypassed fresh support")
	}
	rows := MenuActionsFor(deps, "acme.test", MenuSections(&primary, nil, []api.Action{api.Run("remove", "Remove", nil)}))
	if len(rows) != 1 || !rows[0].MenuOnly {
		t.Fatal("secondary became primary")
	}
}
