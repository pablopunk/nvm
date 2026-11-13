package manager

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"testing"
)

func TestOperationRequirementsUseBoundExecutors(t *testing.T) {
	commands := fake.NewCommands()
	m := &Manager{serviceDeps: services.Deps{Shell: fake.NewShell(), Capabilities: platform.NewCapabilities(platform.CapabilityShellQuickLook), Desktops: real.NewDesktopSwitcher("linux", commands)}}
	if !m.OperationsSupported(nil) || !m.OperationsSupported([]api.Operation{api.OperationFilePreview}) {
		t.Fatal("legacy or preview support lost")
	}
	if m.OperationsSupported([]api.Operation{api.OperationFilePreview, api.OperationSwitchDesktop}) {
		t.Fatal("requirements must be all-of")
	}
	if m.OperationsSupported([]api.Operation{"unknown.operation"}) {
		t.Fatal("unknown operation reported support")
	}
	if len(commands.Calls()) != 0 {
		t.Fatal("discovery ran a process")
	}
	m.commands.seed("acme.test", CommandResult{Commands: []api.Command{{ID: "switch", Title: "Switch", Requires: []api.Operation{api.OperationSwitchDesktop}}}})
	_, failure, err := m.RunCommand(context.Background(), "acme.test", api.Invocation{Ref: api.ActionRef{ID: "switch"}})
	if err != nil || failure == nil || failure.Code != api.CodeUnavailable {
		t.Fatalf("saved invocation did not explain unavailability: %v %v", failure, err)
	}
	result, _ := m.commands.lookup("acme.test")
	if len(result.Commands) != 1 {
		t.Fatal("projection erased cached declaration")
	}
}

func TestOperationRequirementsDoNotBlockCollidingViewActionID(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.settings", fixtureContents(t, "acme.settings"), true); err != nil {
		t.Fatal(err)
	}
	h.waitRunning("acme.settings")
	h.m.commands.seed("acme.settings", CommandResult{Commands: []api.Command{{ID: "shared", Title: "Command", Requires: []api.Operation{api.OperationFilePreview}}}})
	calls := 0
	h.m.mu.Lock()
	entry := h.m.entries["acme.settings"]
	entry.runtime = runCallOverride{runner: entry.runtime, call: func(context.Context, string, string, *api.Invocation) (protocol.Response, error) {
		calls++
		return protocol.Response{Data: []byte(`{"toast":{"message":"view action ran"}}`)}, nil
	}}
	h.m.mu.Unlock()
	invocation := api.Invocation{Ref: api.ActionRef{ID: "shared"}}
	_, failure, err := h.m.RunCommand(context.Background(), "acme.settings", invocation)
	if err != nil || failure == nil || failure.Code != api.CodeUnavailable || calls != 0 {
		t.Fatalf("unsupported command dispatched: %v %v calls=%d", failure, err, calls)
	}
	result, failure, err := h.m.Run(context.Background(), "acme.settings", invocation)
	if err != nil || failure != nil || calls != 1 || result.Toast == nil || result.Toast.Message != "view action ran" {
		t.Fatalf("unrelated view action blocked: %+v %v %v calls=%d", result, failure, err, calls)
	}
}
