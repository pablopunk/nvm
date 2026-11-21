package manager

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"testing"
)

func TestSystemActionRequiresItsOriginalEnabledInstallation(t *testing.T) {
	h := newHarness(t)
	set := fake.NewSet()
	h.m.serviceDeps.SettingsPanes = func() []api.SettingsPane { return []api.SettingsPane{{ID: "known-pane"}} }
	h.m.serviceDeps.SystemCommands = systemfeat.NewServices(systemfeat.Options{Ports: set.Ports()})
	if err := h.m.Install("acme.good", fixtureContents(t, "acme.good"), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	identity := h.m.SnapshotIdentity("acme.good")
	run := func(actor any) error {
		return h.m.SystemCommandAction(context.Background(), "acme.good", "open-system-settings", "known-pane", actor)
	}
	if err := run(identity); err != nil {
		t.Fatal(err)
	}
	if err := run(new(byte)); err == nil {
		t.Fatal("forged installation reached native port")
	}
	if err := h.m.Disable("acme.good"); err != nil {
		t.Fatal(err)
	}
	if err := run(identity); err == nil {
		t.Fatal("disabled installation reached native port")
	}
	if err := h.m.Enable("acme.good"); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	if err := run(identity); err == nil {
		t.Fatal("expired installation revived")
	}
	if err := run(h.m.SnapshotIdentity("acme.good")); err != nil {
		t.Fatal(err)
	}
	if len(set.System.Calls()) != 2 || set.System.Calls()[0].Arg != "known-pane" || set.System.Calls()[1].Arg != "known-pane" {
		t.Fatalf("effects=%+v", set.System.Calls())
	}
	if !h.m.ActionSupported(api.SystemOp("Sleep", "sleep")) || h.m.ActionSupported(api.SystemOp("Keyboard", "open-keyboard-settings")) {
		t.Fatal("action support differs from executor")
	}
	if !h.m.ActionSupported(api.SystemOp("Restart Nevermind", "restart-app")) {
		t.Fatal("app lifecycle route lost")
	}
}
