package manager

import (
	"context"
	"encoding/json"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"testing"
)

func TestShellActionRequiresOriginalEnabledInstallation(t *testing.T) {
	h := newHarness(t)
	commands := fake.NewCommands()
	commands.Script("tool", fake.HelperScript{}, fake.HelperScript{})
	h.m.serviceDeps.Commands = commands
	h.m.serviceDeps.OS = "windows"
	contents := fixtureContents(t, "acme.good")
	var manifest map[string]any
	if err := json.Unmarshal(contents["extension.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["capabilities"] = []string{"desktop.apps", "system"}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	contents["extension.json"] = raw
	if err := h.m.Install("acme.good", contents, true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.good")
	identity := h.m.SnapshotIdentity("acme.good")
	run := func(actor any) error {
		_, err := h.m.ShellAction(context.Background(), "acme.good", api.ShellCommand{Command: "tool"}, actor)
		return err
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
		t.Fatal("old identity revived")
	}
	if err := run(h.m.SnapshotIdentity("acme.good")); err != nil {
		t.Fatal(err)
	}
	if len(commands.Calls()) != 2 {
		t.Fatalf("effects=%+v", commands.Calls())
	}
	if !h.m.ActionSupported(api.Action{Kind: api.ActionShellExec, Shell: &api.ShellCommand{Command: "tool"}}) || h.m.ActionSupported(api.Action{Kind: api.ActionShellExec, Shell: &api.ShellCommand{Script: "echo hello"}}) {
		t.Fatal("support diverged from executor")
	}
}
