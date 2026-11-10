package services

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"testing"
)

func TestSystemDeclarationPolicyAndLifetimeRemainHostOwned(t *testing.T) {
	set := fake.NewSet()
	set.Capabilities = platform.NewCapabilities(platform.CapabilitySystemSleep)
	binding := systemfeat.NewServices(systemfeat.Options{Ports: set.Ports()})
	h := desktopHandler(t, Deps{SystemCommands: binding})
	if !h.publicCapability("system.sleep") || h.publicCapability("system.restart") || len(set.System.Calls()) != 0 {
		t.Fatal("alias support differs from executor or executed a command")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.SystemCommandAction(ctx, "sleep"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(set.System.Calls()) != 0 {
		t.Fatal("cancelled intent reached native effect")
	}
	if err := h.SystemCommandAction(context.Background(), "sleep"); err != nil {
		t.Fatal("existing declarative policy gained a manifest prerequisite:", err)
	}
	_ = h.Close()
	if err := h.SystemCommandAction(context.Background(), "sleep"); !errors.Is(err, api.ErrNotAvailable) {
		t.Fatal(err)
	}
	if len(set.System.Calls()) != 1 {
		t.Fatal("closed host admitted another effect")
	}
}
