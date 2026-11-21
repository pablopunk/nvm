package services

import (
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// switchHost is a window-management handler whose osascript is a fake, on the named OS.
func switchHost(t *testing.T, goos string) (*fake.Commands, *Handler) {
	t.Helper()
	_, deps := newDesktopHost(t, platform.CapabilityWindowManagement)
	commands := fake.NewCommands()
	deps.Commands = commands
	deps.OS = goos
	deps.Desktops = real.NewDesktopSwitcher(goos, commands)
	return commands, desktopHandler(t, deps, api.CapabilityWindowManagement)
}

func switchDesktop(t *testing.T, h *Handler, direction string) api.FocusResult {
	t.Helper()
	var result api.FocusResult
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopWindows, protocol.OpSwitchDesktop, protocol.SwitchDesktopArgs{Direction: direction})), &result); err != nil {
		t.Fatalf("decode focus result: %v", err)
	}
	return result
}

// TestNextDesktopSendsTheMissionControlRightArrow: the next desktop is the right arrow with Control held, as 1.x sent it.
func TestNextDesktopSendsTheMissionControlRightArrow(t *testing.T) {
	commands, h := switchHost(t, "darwin")
	commands.Script("osascript", fake.HelperScript{})
	if result := switchDesktop(t, h, "next"); !result.OK {
		t.Fatalf("result = %+v, want ok", result)
	}
	args := strings.Join(commands.Calls()[0].Args, " ")
	if !strings.Contains(args, "key code 124 using control down") {
		t.Fatalf("osascript args = %q, want the right arrow with Control", args)
	}
}

func TestPreviousDesktopSendsTheLeftArrow(t *testing.T) {
	commands, h := switchHost(t, "darwin")
	commands.Script("osascript", fake.HelperScript{})
	switchDesktop(t, h, "previous")
	if args := strings.Join(commands.Calls()[0].Args, " "); !strings.Contains(args, "key code 123 using control down") {
		t.Fatalf("osascript args = %q, want the left arrow with Control", args)
	}
}

func TestRefusedKeysReturnThePaletteReadyReason(t *testing.T) {
	commands, h := switchHost(t, "darwin")
	commands.Script("osascript", fake.HelperScript{ExitCode: 1, Stderr: []byte("not allowed\n")})
	result := switchDesktop(t, h, "next")
	if result.OK || result.Reason != "failed" || !strings.HasPrefix(result.Message, "Could not switch desktop.") {
		t.Fatalf("result = %+v, want the failure message", result)
	}
}

func TestSwitchDesktopOffMacOSIsUnsupported(t *testing.T) {
	commands, h := switchHost(t, "linux")
	reply := serve(t, h, protocol.ServiceDesktopWindows, protocol.OpSwitchDesktop, protocol.SwitchDesktopArgs{Direction: "next"})
	if reply.Failure == nil || reply.Failure.Code != api.CodeUnsupported {
		t.Fatalf("reply = %+v, want an unsupported failure", reply)
	}
	if len(commands.Calls()) != 0 {
		t.Fatalf("osascript ran off macOS: %+v", commands.Calls())
	}
}

func TestUnknownDesktopDirectionIsAnInvalidSpec(t *testing.T) {
	commands, h := switchHost(t, "darwin")
	if result := switchDesktop(t, h, "sideways"); result.OK || result.Reason != "invalid-spec" {
		t.Fatalf("result = %+v, want invalid-spec", result)
	}
	if len(commands.Calls()) != 0 {
		t.Fatal("osascript ran for an unknown direction")
	}
}

func TestDesktopSwitchProbeMatchesItsBoundExecutor(t *testing.T) {
	for _, goos := range []string{"darwin", "linux", "windows"} {
		commands, h := switchHost(t, goos)
		if h.publicCapability("windows.switch-desktop") != (goos == "darwin") {
			t.Fatalf("incorrect support on %s", goos)
		}
		if !h.publicCapability("window-management") {
			t.Fatal("operation support changed legacy family probe")
		}
		if len(commands.Calls()) != 0 {
			t.Fatal("discovery executed a process")
		}
	}
}
