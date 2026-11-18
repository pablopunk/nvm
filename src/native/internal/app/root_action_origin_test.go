package app

import (
	"context"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/search"
)

// TestReplacedInstallationCannotRunAnOlderAnswersAction is the cached-row sequence: an answer is produced by one
// installation, the extension is replaced, and only then is the answer converted to a row. The row's action must
// stay bound to the installation that answered, so it fails closed instead of running against the replacement.
func TestReplacedInstallationCannotRunAnOlderAnswersAction(t *testing.T) {
	const id = "acme.good"
	m := newManagerOver(t, nil, id)
	waitForState(t, m, id, supervisor.StateRunning)
	ctx, cancel := context.WithTimeout(context.Background(), fixtureWaitTimeout)
	defer cancel()
	var origin any
	for _, result := range m.Search(ctx, "a") {
		if result.ExtensionID == id {
			origin = result.Origin
		}
	}
	if origin == nil {
		t.Fatal("the extension did not answer with its installation identity")
	}
	if err := m.Disable(id); err != nil {
		t.Fatal(err)
	}
	if err := m.Enable(id); err != nil {
		t.Fatal(err)
	}
	waitForState(t, m, id, supervisor.StateRunning)

	var runs int
	countRun := func(search.Host) { runs++ }
	host := &recordingHost{}
	row := guardRootExtensionActions(m, id, origin, search.Item{Actions: []search.Action{{Run: countRun}}})
	row.Actions[0].Run(host)
	if runs != 0 || len(host.toasts) == 0 {
		t.Fatal("an answer from the replaced installation ran against its replacement")
	}
	for _, result := range m.Search(ctx, "a") {
		if result.ExtensionID == id {
			fresh := guardRootExtensionActions(m, id, result.Origin, search.Item{Actions: []search.Action{{Run: countRun}}})
			fresh.Actions[0].Run(host)
		}
	}
	if runs != 1 {
		t.Fatal("the current installation's answer was rejected")
	}
}
