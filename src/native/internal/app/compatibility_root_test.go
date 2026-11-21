package app

import (
	"testing"

	updatefake "github.com/pablopunk/nvm/native/internal/update/fake"
)

// A backend that refuses this client refuses its AI and sign-in requests with the update message, and accepts others.
func TestRefusedBackendFailsRequestsWithTheUpdateMessage(t *testing.T) {
	p, _ := wiredApp(t)
	p.compat.record(connectedBase, false)
	if err := p.refusalFor(connectedBase); err != errClientRefused {
		t.Fatalf("refusal = %v, want the update message", err)
	}
	p.compat.record(connectedBase, true)
	if err := p.refusalFor(connectedBase); err != nil {
		t.Fatalf("accepted backend refused: %v", err)
	}
	if err := p.refusalFor("https://unknown.example"); err != nil {
		t.Fatalf("unknown backend refused before any manifest: %v", err)
	}
}

// While the backend refuses this client the root shows one row, in place of the update row, and its click runs the
// updater's action.
func TestRootShowsTheCompatibilityRowInPlaceOfTheUpdateRow(t *testing.T) {
	m := newMenuUpdates(t, updatefake.NewEngine(nil))
	m.app.compat.setActive(connectedBase)
	m.app.compat.record(connectedBase, false)
	rows := updateRootRows(m.app, nil, "")
	if len(rows) != 1 || rows[0].Title != "Update Nevermind" {
		t.Fatalf("root rows = %+v, want one Update Nevermind row", rows)
	}
	if len(rows[0].Actions) != 1 {
		t.Fatalf("idle root row has %d actions, want the check action", len(rows[0].Actions))
	}
	m.onUIThread(func() { rows[0].Actions[0].Run(&recordingHost{}) })
	if title, shown := m.topPage(); title != "Updates" || !shown {
		t.Fatalf("top page = %q shown %v, want the Updates page", title, shown)
	}
}

// Without a refusal the root keeps the update rows as they were: none for an idle updater.
func TestRootHasNoCompatibilityRowWhileAccepted(t *testing.T) {
	m := newMenuUpdates(t, updatefake.NewEngine(nil))
	m.app.compat.setActive(connectedBase)
	m.app.compat.record(connectedBase, true)
	if rows := updateRootRows(m.app, nil, ""); len(rows) != 0 {
		t.Fatalf("accepted backend root rows = %+v, want none for an idle updater", rows)
	}
}
