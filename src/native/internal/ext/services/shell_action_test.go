package services

import (
	"context"
	"errors"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"strings"
	"testing"
)

func TestShellActionSharesDeclarationAndProcessBudget(t *testing.T) {
	commands, denied := processHost(t, "linux")
	if _, err := denied.ShellAction(context.Background(), api.ShellCommand{Command: "tool"}); err == nil || failureOf(err).Code != api.CodePermission {
		t.Fatalf("declaration: %v", err)
	}
	if len(commands.Calls()) != 0 {
		t.Fatal("undeclared process")
	}
	commands, host := processHost(t, "linux", api.CapabilitySystem)
	commands.Script("tool", make([]fake.HelperScript, processRunsPerMinute)...)
	for i := 0; i < processRunsPerMinute; i++ {
		runProcessOp(t, host, protocol.OpExec, protocol.ShellExecArgs{Command: "tool"})
	}
	if _, err := host.ShellAction(context.Background(), api.ShellCommand{Command: "tool"}); err == nil || failureOf(err).Code != api.CodeQuota {
		t.Fatalf("shared budget: %v", err)
	}
	if len(commands.Calls()) != processRunsPerMinute {
		t.Fatal("action escaped direct-call budget")
	}
}

func TestShellActionPreservesArgumentsOptionsAndNonzeroOutput(t *testing.T) {
	commands, host := processHost(t, "windows", api.CapabilitySystem)
	commands.Script("tool", fake.HelperScript{Stdout: []byte("hello world"), Stderr: []byte("failure"), ExitCode: 7})
	result, err := host.ShellAction(context.Background(), api.ShellCommand{Command: "tool", Args: []string{"a b", "$(literal)"}, CWD: "~/work", Env: map[string]string{"MODE": "fixture"}, OutputLimit: 5})
	if err != nil || result.ExitCode != 7 || result.Stdout != "hello"+truncatedOutputMarker || result.Stderr != "failu"+truncatedOutputMarker {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	calls := commands.Calls()
	if len(calls) != 1 || calls[0].Binary != "tool" || strings.Join(calls[0].Args, "|") != "a b|$(literal)" || calls[0].Dir != "/Users/acme/work" || calls[0].Env["MODE"] != "fixture" {
		t.Fatalf("calls=%+v", calls)
	}
}

func TestShellActionAdmissionAndUnsupportedScriptHaveNoEffects(t *testing.T) {
	for _, mode := range []string{"cancelled", "closed", "windows-script"} {
		t.Run(mode, func(t *testing.T) {
			commands, host := processHost(t, "windows", api.CapabilitySystem)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			command := api.ShellCommand{Command: "tool"}
			switch mode {
			case "cancelled":
				cancel()
			case "closed":
				_ = host.Close()
			case "windows-script":
				command = api.ShellCommand{Script: "echo hello"}
			}
			_, err := host.ShellAction(ctx, command)
			if err == nil {
				t.Fatal("refusal missing")
			}
			if mode == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel: %v", err)
			}
			if mode == "windows-script" && failureOf(err).Code != api.CodeUnavailable {
				t.Fatalf("unsupported: %v", err)
			}
			if len(commands.Calls()) != 0 {
				t.Fatal("refused operation reached process port")
			}
		})
	}
	commands, host := processHost(t, "windows", api.CapabilitySystem)
	requireCode(t, serve(t, host, protocol.ServiceDesktopShell, protocol.OpExec, protocol.ShellExecArgs{Command: "tool", Options: api.ShellOptions{Shell: true}}), api.CodeUnavailable, "POSIX")
	if len(commands.Calls()) != 0 {
		t.Fatal("POSIX shell invoked on Windows")
	}
}
