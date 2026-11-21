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
	if err := h.SystemCommandAction(ctx, "sleep", ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(set.System.Calls()) != 0 {
		t.Fatal("cancelled intent reached native effect")
	}
	if err := h.SystemCommandAction(context.Background(), "sleep", ""); err != nil {
		t.Fatal("existing declarative policy gained a manifest prerequisite:", err)
	}
	_ = h.Close()
	if err := h.SystemCommandAction(context.Background(), "sleep", ""); !errors.Is(err, api.ErrNotAvailable) {
		t.Fatal(err)
	}
	if len(set.System.Calls()) != 1 {
		t.Fatal("closed host admitted another effect")
	}
}

func TestSystemSettingsPaneValidatesOfferedTargetAndFinalAdmission(t *testing.T) {
	for _, mode := range []string{"valid", "unknown", "lookup-cancelled", "lookup-closed", "unsupported"} {
		t.Run(mode, func(t *testing.T) {
			set := fake.NewSet()
			if mode == "unsupported" {
				set.Capabilities = platform.NewCapabilities()
			}
			binding := systemfeat.NewServices(systemfeat.Options{Ports: set.Ports()})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var host *Handler
			host = desktopHandler(t, Deps{SystemCommands: binding, SettingsPanes: func() []api.SettingsPane {
				if mode == "lookup-cancelled" {
					cancel()
				}
				if mode == "lookup-closed" {
					_ = host.Close()
				}
				return []api.SettingsPane{{ID: "known-pane", Title: "Known"}}
			}})
			pane := "known-pane"
			if mode == "unknown" {
				pane = "forged-pane"
			}
			err := host.SystemCommandAction(ctx, "open-system-settings", pane)
			if mode == "valid" {
				calls := set.System.Calls()
				if err != nil || len(calls) != 1 || calls[0].Op != fake.OpSystemOpenSettings || calls[0].Arg != "known-pane" {
					t.Fatalf("calls=%+v err=%v", calls, err)
				}
			} else {
				if err == nil || len(set.System.Calls()) != 0 {
					t.Fatalf("refusal missing: %v %+v", err, set.System.Calls())
				}
				if mode == "lookup-cancelled" && !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel: %v", err)
				}
			}
		})
	}
}
