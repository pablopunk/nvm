package manager

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func TestWindowActionRequiresOriginalEnabledInstallation(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	identity := h.m.SnapshotIdentity("acme.good")
	effects := 0
	run := func(actor any) error {
		return h.m.WindowAction("acme.good", api.Action{Kind: api.ActionWindow}, actor, func(string, api.Action) error {
			effects++
			return nil
		})
	}
	if err := run(identity); err != nil {
		t.Fatal(err)
	}
	if err := run(new(byte)); err == nil {
		t.Fatal("forged identity admitted")
	}
	if err := h.m.Disable("acme.good"); err != nil {
		t.Fatal(err)
	}
	if err := run(identity); err == nil {
		t.Fatal("disabled identity admitted")
	}
	if err := h.m.Enable("acme.good"); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	if err := run(identity); err == nil {
		t.Fatal("old identity admitted")
	}
	if err := run(h.m.SnapshotIdentity("acme.good")); err != nil {
		t.Fatal(err)
	}
	if effects != 2 {
		t.Fatalf("effects = %d", effects)
	}
}
