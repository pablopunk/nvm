package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// notifiedHost is a process host whose Notify port records each message the user would see.
func notifiedHost(t *testing.T, os string) (*fake.Commands, *Handler, *[][2]string) {
	t.Helper()
	_, deps := newDesktopHost(t)
	commands := fake.NewCommands()
	deps.Commands = commands
	deps.OS = os
	shown := &[][2]string{}
	deps.Notify = func(title, message string) { *shown = append(*shown, [2]string{title, message}) }
	return commands, desktopHandler(t, deps, api.CapabilitySystem), shown
}

// runScript runs one AppleScript through the handler, as an extension's RunAction does.
func runScript(t *testing.T, h *Handler) api.ShellResult {
	t.Helper()
	return runProcessOp(t, h, protocol.OpAppleScript, protocol.ShellScriptArgs{Script: `tell application "System Events" to return true`})
}

// TestRefusedAutomationGrantIsShownInTheHUD: a theme toggle that ignores its exit code still tells the user why it did
// nothing: the Automation grant for System Events is missing, and the HUD says where to allow it.
func TestRefusedAutomationGrantIsShownInTheHUD(t *testing.T) {
	commands, h, shown := notifiedHost(t, "darwin")
	commands.Script("osascript", fake.HelperScript{
		Stderr:   []byte("execution error: Not authorized to send Apple events to System Events. (-1743)\n"),
		ExitCode: 1,
	})
	result := runScript(t, h)
	if result.ExitCode != 1 {
		t.Fatalf("the extension got exit %d; want the program's own exit code", result.ExitCode)
	}
	if len(*shown) != 1 || (*shown)[0][1] != automationHint {
		t.Fatalf("the HUD showed %q; want the Automation hint once", *shown)
	}
	if (*shown)[0][0] == "" {
		t.Fatal("the HUD message has no extension title")
	}
}

// TestOtherAppleScriptFailureShowsItsFirstLine: a script error reaches the HUD with its first line, not the exit code.
func TestOtherAppleScriptFailureShowsItsFirstLine(t *testing.T) {
	commands, h, shown := notifiedHost(t, "darwin")
	commands.Script("osascript", fake.HelperScript{Stderr: []byte("\n\nsyntax error: Expected end of line\nmore\n"), ExitCode: 1})
	runScript(t, h)
	if len(*shown) != 1 || (*shown)[0][1] != "AppleScript failed: syntax error: Expected end of line" {
		t.Fatalf("the HUD showed %q; want the first line of the error", *shown)
	}
}

// TestAFailedScriptWithoutTextShowsItsExitCode: nothing printed still leaves a reason the user can read.
func TestAFailedScriptWithoutTextShowsItsExitCode(t *testing.T) {
	commands, h, shown := notifiedHost(t, "darwin")
	commands.Script("osascript", fake.HelperScript{ExitCode: 2})
	runScript(t, h)
	if len(*shown) != 1 || (*shown)[0][1] != "AppleScript failed with exit code 2" {
		t.Fatalf("the HUD showed %q; want the exit code", *shown)
	}
}

// TestAStartFailureIsShownToo: a program that cannot start is a failure the user must see.
func TestAStartFailureIsShownToo(t *testing.T) {
	commands, h, shown := notifiedHost(t, "darwin")
	commands.Script("osascript", fake.HelperScript{Err: errors.New("executable file not found")})
	result := runScript(t, h)
	if result.ExitCode != 1 || len(*shown) != 1 || !strings.Contains((*shown)[0][1], "executable file not found") {
		t.Fatalf("result %+v, HUD %q; want exit 1 and the start error in the HUD", result, *shown)
	}
}

// TestAppleScriptOffMacOSIsNeverShown: on another OS the extension gets the failure in its result, and the user sees
// nothing: a macOS feature missing on Linux is not news to them.
func TestAppleScriptOffMacOSIsNeverShown(t *testing.T) {
	_, h, shown := notifiedHost(t, "linux")
	runScript(t, h)
	if len(*shown) != 0 {
		t.Fatalf("the HUD showed %q; want nothing off macOS", *shown)
	}
}

// TestSuccessfulAppleScriptShowsNothing: a script that works makes no noise.
func TestSuccessfulAppleScriptShowsNothing(t *testing.T) {
	commands, h, shown := notifiedHost(t, "darwin")
	commands.Script("osascript", fake.HelperScript{Stdout: []byte("true\n")})
	runScript(t, h)
	if len(*shown) != 0 {
		t.Fatalf("a successful script showed %q", *shown)
	}
}
