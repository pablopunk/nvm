package manager

import (
	"context"
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
	executor := &windowActionRecorder{}
	run := func(actor any) error {
		return h.m.WindowAction(context.Background(), "acme.good", api.Action{Kind: api.ActionWindow, Op: "show", Window: &api.WindowRef{ID: "sample"}}, actor, executor)
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
	if len(executor.owners) != 2 || executor.owners[0] != "acme.good" || executor.owners[1] != "acme.good" {
		t.Fatalf("effects = %v", executor.owners)
	}
}

type windowActionRecorder struct{ owners []string }

func (r *windowActionRecorder) Perform(owner string, _ api.Action) error {
	r.owners = append(r.owners, owner)
	return nil
}

func TestWindowActionBuiltinMustBeRegisteredAndHostLive(t *testing.T) {
	h := newHarness(t, withBuiltIns(builtinSample{}))
	executor := &windowActionRecorder{}
	action := api.Action{Kind: api.ActionWindow, Op: "show", Window: &api.WindowRef{ID: "sample"}}
	if err := h.m.WindowAction(context.Background(), "nevermind.sample", action, nil, executor); err != nil {
		t.Fatal(err)
	}
	if err := h.m.WindowAction(context.Background(), "nevermind.forged", action, nil, executor); err == nil {
		t.Fatal("unregistered builtin admitted")
	}
	if err := h.m.WindowAction(context.Background(), "nevermind.sample", api.Action{Kind: api.ActionCopyText}, nil, executor); err == nil {
		t.Fatal("non-window action reached executor")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.m.WindowAction(ctx, "nevermind.sample", action, nil, executor); err == nil {
		t.Fatal("cancelled host effect admitted")
	}
	h.m.mu.Lock()
	entry, err := h.m.findLocked("nevermind.sample")
	h.m.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	_ = entry.host.Close()
	if err := h.m.WindowAction(context.Background(), "nevermind.sample", action, nil, executor); err == nil {
		t.Fatal("closed host admitted")
	}
	if len(executor.owners) != 1 {
		t.Fatalf("effects=%v", executor.owners)
	}
}
