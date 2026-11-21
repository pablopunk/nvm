package system

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"testing"
)

func TestSystemBindingUsesOneSupportAndEffectContract(t *testing.T) {
	set := fake.NewSet()
	set.Capabilities = platform.NewCapabilities(platform.CapabilitySystemSleep)
	service := servicesOver("linux", set)
	if !service.Supported(CommandSleep) || !service.SupportsCapability(platform.CapabilitySystemSleep) || service.Supported(CommandRestart) || service.Supported("open-keyboard-settings") {
		t.Fatal("incorrect structural support")
	}
	if len(set.System.Calls()) != 0 {
		t.Fatal("support lookup ran a native operation")
	}
	if err := service.Run(CommandRestart); !errors.Is(err, platform.ErrUnsupported) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.RunContext(ctx, CommandSleep); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(set.System.Calls()) != 0 {
		t.Fatal("unsupported or cancelled operation reached native port")
	}
	set.System.FailNext(fake.OpSystemSleep, platform.ErrDenied)
	if err := service.Run(CommandSleep); !errors.Is(err, platform.ErrDenied) {
		t.Fatal(err)
	}
	if !service.Supported(CommandSleep) {
		t.Fatal("permission denial became missing implementation")
	}
	if len(set.System.Calls()) != 0 {
		t.Fatal("native effect was replayed after denial")
	}
	if err := service.Run(CommandSleep); err != nil {
		t.Fatal(err)
	}
	if len(set.System.Calls()) != 1 {
		t.Fatal("supported intent did not dispatch once")
	}
	missing := NewServices(Options{Ports: platform.Ports{Capabilities: set.Capabilities}})
	if missing.Supported(CommandSleep) || !errors.Is(missing.Run(CommandSleep), platform.ErrUnsupported) {
		t.Fatal("missing executor was supported")
	}
}
