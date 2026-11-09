package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// processHost is a desktop handler whose command runner is a fake the test scripts.
func processHost(t *testing.T, os string, capabilities ...api.Capability) (*fake.Commands, *Handler) {
	t.Helper()
	_, deps := newDesktopHost(t)
	commands := fake.NewCommands()
	deps.Commands = commands
	deps.OS = os
	deps.Home = "/Users/acme"
	return commands, desktopHandler(t, deps, capabilities...)
}

// runProcessOp sends one process call and decodes its shell result.
func runProcessOp(t *testing.T, h *Handler, op string, args any) api.ShellResult {
	t.Helper()
	var result api.ShellResult
	if err := jsonInto(mustOK(t, serve(t, h, protocol.ServiceDesktopShell, op, args)), &result); err != nil {
		t.Fatalf("decode shell result: %v", err)
	}
	return result
}

func TestProcessOperationsNeedTheSystemCapability(t *testing.T) {
	commands, h := processHost(t, "darwin", api.CapabilityDesktopFiles)
	requireCode(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpExec, protocol.ShellExecArgs{Command: "gh"}), api.CodePermission, "system")
	requireCode(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpScript, protocol.ShellScriptArgs{Script: "true"}), api.CodePermission, "system")
	requireCode(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpWhich, protocol.ShellWhichArgs{Command: "gh"}), api.CodePermission, "system")
	if calls := commands.Calls(); len(calls) != 0 {
		t.Fatalf("an undeclared extension ran %d processes", len(calls))
	}
}

func TestExecRunsTheProgramAndExpandsHomeInItsArguments(t *testing.T) {
	commands, h := processHost(t, "darwin", api.CapabilitySystem)
	commands.Script("/Users/acme/bin/tool", fake.HelperScript{Stdout: []byte("ok\n")})
	result := runProcessOp(t, h, protocol.OpExec, protocol.ShellExecArgs{Command: "~/bin/tool", Args: []string{"~", "plain"}})
	if result.Stdout != "ok\n" || result.ExitCode != 0 {
		t.Fatalf("result = %+v; want stdout ok and exit 0", result)
	}
	call := commands.Calls()[0]
	if call.Binary != "/Users/acme/bin/tool" || strings.Join(call.Args, "|") != "/Users/acme|plain" {
		t.Fatalf("ran %s %q; want the home folder expanded in the command and arguments", call.Binary, call.Args)
	}
}

func TestShellOptionJoinsTheCommandForTheShell(t *testing.T) {
	commands, h := processHost(t, "darwin", api.CapabilitySystem)
	commands.Script("/bin/sh", fake.HelperScript{})
	runProcessOp(t, h, protocol.OpExec, protocol.ShellExecArgs{Command: "echo", Args: []string{"a", "b"}, Options: api.ShellOptions{Shell: true}})
	call := commands.Calls()[0]
	if call.Binary != "/bin/sh" || strings.Join(call.Args, "|") != "-c|echo a b" {
		t.Fatalf("ran %s %q; want /bin/sh -c 'echo a b'", call.Binary, call.Args)
	}
}

func TestExecPassesWorkingFolderAndEnvironment(t *testing.T) {
	commands, h := processHost(t, "darwin", api.CapabilitySystem)
	commands.Script("tool", fake.HelperScript{})
	runProcessOp(t, h, protocol.OpExec, protocol.ShellExecArgs{Command: "tool", Options: api.ShellOptions{CWD: "~/work", Env: map[string]string{"MODE": "fast"}}})
	call := commands.Calls()[0]
	if call.Dir != "/Users/acme/work" || call.Env["MODE"] != "fast" {
		t.Fatalf("call = %+v; want the expanded working folder and the extra variable", call)
	}
}

func TestScriptRunsUnderALoginBash(t *testing.T) {
	commands, h := processHost(t, "darwin", api.CapabilitySystem)
	commands.Script("/bin/bash", fake.HelperScript{Stdout: []byte("done")})
	result := runProcessOp(t, h, protocol.OpScript, protocol.ShellScriptArgs{Script: "cp ~/a ~/b"})
	call := commands.Calls()[0]
	if result.Stdout != "done" || call.Binary != "/bin/bash" || strings.Join(call.Args, "|") != "-lc|cp /Users/acme/a /Users/acme/b" {
		t.Fatalf("script call %s %q with result %+v; want a login bash with the home folder expanded", call.Binary, call.Args, result)
	}
}

func TestAProcessKilledAtItsDeadlineReportsExitMinusOne(t *testing.T) {
	commands, h := processHost(t, "darwin", api.CapabilitySystem)
	commands.Script("slow", fake.HelperScript{Timeout: true})
	result := runProcessOp(t, h, protocol.OpExec, protocol.ShellExecArgs{Command: "slow", Options: api.ShellOptions{TimeoutMs: 1}})
	if result.ExitCode != -1 || !strings.Contains(result.Stderr, "Killed after 1000ms") {
		t.Fatalf("result = %+v; want exit -1 and a kill note, with the timeout clamped to one second", result)
	}
}

func TestOutputBeyondTheLimitIsCutAndMarked(t *testing.T) {
	commands, h := processHost(t, "darwin", api.CapabilitySystem)
	commands.Script("noisy", fake.HelperScript{Stdout: []byte(strings.Repeat("x", 5000))})
	result := runProcessOp(t, h, protocol.OpExec, protocol.ShellExecArgs{Command: "noisy", Options: api.ShellOptions{OutputLimit: 1000}})
	if !strings.HasPrefix(result.Stdout, strings.Repeat("x", 1000)) || !strings.HasSuffix(result.Stdout, truncatedOutputMarker) || len(result.Stdout) != 1000+len(truncatedOutputMarker) {
		t.Fatalf("stdout has %d bytes; want 1000 kept and the truncation marker", len(result.Stdout))
	}
}

func TestAProgramThatCannotStartIsExitOne(t *testing.T) {
	commands, h := processHost(t, "darwin", api.CapabilitySystem)
	commands.Script("missing", fake.HelperScript{Err: errors.New("executable file not found")})
	result := runProcessOp(t, h, protocol.OpExec, protocol.ShellExecArgs{Command: "missing"})
	if result.ExitCode != 1 || !strings.Contains(result.Stderr, "executable file not found") {
		t.Fatalf("result = %+v; want exit 1 and the start error", result)
	}
}

func TestAppleScriptRunsOsascriptOnMacOSOnly(t *testing.T) {
	commands, h := processHost(t, "darwin", api.CapabilitySystem)
	commands.Script("osascript", fake.HelperScript{Stdout: []byte("true\n")})
	result := runProcessOp(t, h, protocol.OpAppleScript, protocol.ShellScriptArgs{Script: "return true"})
	call := commands.Calls()[0]
	if result.Stdout != "true\n" || call.Binary != "osascript" || strings.Join(call.Args, "|") != "-e|return true" {
		t.Fatalf("applescript call %s %q with result %+v", call.Binary, call.Args, result)
	}

	offMac, other := processHost(t, "linux", api.CapabilitySystem)
	result = runProcessOp(t, other, protocol.OpAppleScript, protocol.ShellScriptArgs{Script: "return true"})
	if result.ExitCode != 1 || result.Stderr != appleScriptUnavailable || len(offMac.Calls()) != 0 {
		t.Fatalf("off macOS result = %+v; want the unavailable answer and no process", result)
	}
}

func TestWhichTrimsTheFoundPath(t *testing.T) {
	commands, h := processHost(t, "darwin", api.CapabilitySystem)
	commands.Script("/usr/bin/which", fake.HelperScript{Stdout: []byte("/opt/homebrew/bin/gh\n")})
	result := runProcessOp(t, h, protocol.OpWhich, protocol.ShellWhichArgs{Command: "gh"})
	if result.Stdout != "/opt/homebrew/bin/gh" || result.ExitCode != 0 {
		t.Fatalf("which = %+v; want the trimmed path", result)
	}
}

func TestProcessOperationsAnswerUnavailableWithoutARunner(t *testing.T) {
	_, deps := newDesktopHost(t)
	h := desktopHandler(t, deps, api.CapabilitySystem)
	requireCode(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpExec, protocol.ShellExecArgs{Command: "gh"}), api.CodeUnavailable, "not available")
}

func TestProcessOperationsAreRateLimited(t *testing.T) {
	commands, h := processHost(t, "darwin", api.CapabilitySystem)
	commands.Script("tool", make([]fake.HelperScript, processRunsPerMinute)...)
	for i := 0; i < processRunsPerMinute; i++ {
		mustOK(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpExec, protocol.ShellExecArgs{Command: "tool"}))
	}
	requireCode(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpExec, protocol.ShellExecArgs{Command: "tool"}), api.CodeQuota, "rate limited")
}

// TestProcessBudgetCoversAScanOfManyApps is Remove App Quarantine on a Mac with a few hundred apps: one xattr run per
// app must fit in the budget the API states, and the refusal past it says what to do instead.
func TestProcessBudgetCoversAScanOfManyApps(t *testing.T) {
	commands, h := processHost(t, "darwin", api.CapabilitySystem)
	const apps = 300
	commands.Script("/usr/bin/xattr", make([]fake.HelperScript, apps)...)
	for i := 0; i < apps; i++ {
		mustOK(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpExec, protocol.ShellExecArgs{Command: "/usr/bin/xattr"}))
	}
	if processRunsPerMinute <= apps {
		t.Fatalf("budget = %d runs per minute, want more than the %d runs of the scan", processRunsPerMinute, apps)
	}
	commands.Script("tool", make([]fake.HelperScript, processRunsPerMinute)...)
	for i := 0; i < processRunsPerMinute-apps; i++ {
		mustOK(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpExec, protocol.ShellExecArgs{Command: "tool"}))
	}
	requireCode(t, serve(t, h, protocol.ServiceDesktopShell, protocol.OpExec, protocol.ShellExecArgs{Command: "tool"}), api.CodeQuota, "run one script that handles many items")
}
