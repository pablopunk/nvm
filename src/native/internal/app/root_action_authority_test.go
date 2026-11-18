package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestCachedExtensionActionRejectsDisabledInstallation(t *testing.T) {
	m := newManagerOver(t, nil, "acme.good")
	waitForState(t, m, "acme.good", supervisor.StateRunning)
	var runs int
	row := guardRootExtensionActions(m, "acme.good", search.Item{Actions: []search.Action{{Run: func(search.Host) { runs++ }}}})
	host := &recordingHost{}
	row.Actions[0].Run(host)
	if runs != 1 {
		t.Fatal("enabled extension action was rejected")
	}
	if err := m.Disable("acme.good"); err != nil {
		t.Fatal(err)
	}
	row.Actions[0].Run(host)
	if runs != 1 || len(host.toasts) == 0 {
		t.Fatal("disabled extension retained action authority")
	}
}
