package real

import (
	"context"
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestDesktopSwitchSupportDoesNotRunAProcess(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			commands := fake.NewCommands()
			switcher := NewDesktopSwitcher(goos, commands)
			if switcher.Supported() != (goos == "darwin") {
				t.Fatal("incorrect structural support")
			}
			if len(commands.Calls()) != 0 {
				t.Fatal("support probe executed a process")
			}
			if goos != "darwin" {
				_, err := switcher.Switch(context.Background(), "next")
				if !errors.Is(err, platform.ErrUnsupported) || len(commands.Calls()) != 0 {
					t.Fatal("unsupported switch reached native effect", err)
				}
			}
		})
	}
	if NewDesktopSwitcher("darwin", nil).Supported() {
		t.Fatal("missing executor reported support")
	}
}

func TestDesktopSwitchCancelledOrInvalidNeverDispatches(t *testing.T) {
	commands := fake.NewCommands()
	switcher := NewDesktopSwitcher("darwin", commands)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := switcher.Switch(ctx, "next"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := switcher.Switch(context.Background(), "sideways"); err == nil {
		t.Fatal("invalid direction accepted")
	}
	if len(commands.Calls()) != 0 {
		t.Fatal("cancelled or invalid switch reached native effect")
	}
}
