package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Process limits and the programs the shell operations run. They match the Electron host's defaults.
const (
	defaultProcessTimeout  = 30 * time.Second
	minProcessTimeout      = time.Second
	maxProcessTimeout      = 120 * time.Second
	defaultOutputLimit     = 200_000
	truncatedOutputMarker  = "\n... output truncated ..."
	posixShellBinary       = "/bin/sh"
	loginScriptShell       = "/bin/bash"
	whichBinary            = "/usr/bin/which"
	osascriptBinary        = "osascript"
	appleScriptUnavailable = "AppleScript is not available on this OS"
)

// homeTilde matches a "~" that stands for the home folder: at the start of a path or before a slash, as the shell does.
var homeTilde = regexp.MustCompile(`~(/|$)`)

// runProcess runs one of the process operations. It needs the system capability, and it never runs a process
// when the host has no command runner.
func (h *Handler) runProcess(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilitySystem); err != nil {
		return nil, err
	}
	if !platform.ProcessAvailability(h.deps.Commands, h.deps.OS).Exec {
		return nil, unavailable("processes are not available in this build")
	}
	if !h.processes.allow() {
		return nil, processBudgetExceeded()
	}
	switch call.Op {
	case protocol.OpExec:
		return h.execProgram(ctx, call.Args)
	case protocol.OpScript:
		return h.runLoginScript(ctx, call.Args)
	case protocol.OpAppleScript:
		return h.runAppleScript(ctx, call.Args)
	case protocol.OpWhich:
		return h.whichProgram(ctx, call.Args)
	}
	return nil, invalid("desktop.shell has no operation %q", call.Op)
}

func (h *Handler) execProgram(ctx context.Context, raw json.RawMessage) (api.ShellResult, error) {
	var args protocol.ShellExecArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.ShellResult{}, err
	}
	if strings.TrimSpace(args.Command) == "" {
		return api.ShellResult{}, invalid("a command is required")
	}
	command := h.expandHome(args.Command)
	programArgs := make([]string, len(args.Args))
	for i, arg := range args.Args {
		programArgs[i] = h.expandHome(arg)
	}
	spec := platform.CommandSpec{Binary: command, Args: programArgs}
	if args.Options.Shell {
		if !platform.ProcessAvailability(h.deps.Commands, h.deps.OS).LoginScript {
			return api.ShellResult{}, unavailable("POSIX shell execution is not available on this system")
		}
		spec = platform.CommandSpec{Binary: posixShellBinary, Args: []string{"-c", strings.Join(append([]string{command}, programArgs...), " ")}}
	}
	return h.run(ctx, spec, args.Options)
}

func (h *Handler) runLoginScript(ctx context.Context, raw json.RawMessage) (api.ShellResult, error) {
	var args protocol.ShellScriptArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.ShellResult{}, err
	}
	if !platform.ProcessAvailability(h.deps.Commands, h.deps.OS).LoginScript {
		return api.ShellResult{}, unavailable("login shell scripts are not available on this system")
	}
	script := h.expandHomeInScript(args.Script)
	return h.run(ctx, platform.CommandSpec{Binary: loginScriptShell, Args: []string{"-lc", script}}, args.Options)
}

// expandHomeInScript replaces every "~" that stands for the home folder in script source.
func (h *Handler) expandHomeInScript(script string) string {
	if h.deps.Home == "" {
		return script
	}
	return homeTilde.ReplaceAllStringFunc(script, func(match string) string {
		return h.deps.Home + match[1:]
	})
}

// runAppleScript runs osascript on macOS. Other systems answer the failure the Electron host gives.
func (h *Handler) runAppleScript(ctx context.Context, raw json.RawMessage) (api.ShellResult, error) {
	var args protocol.ShellScriptArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.ShellResult{}, err
	}
	if h.deps.OS != "darwin" {
		// The extension learns it from the result; the user is never told that another OS's feature is missing.
		return api.ShellResult{Stderr: appleScriptUnavailable, ExitCode: 1}, nil
	}
	result, err := h.run(ctx, platform.CommandSpec{Binary: osascriptBinary, Args: []string{"-e", args.Script}}, args.Options)
	if err == nil && result.ExitCode != 0 {
		h.reportAppleScriptFailure(result)
	}
	return result, err
}

func (h *Handler) whichProgram(ctx context.Context, raw json.RawMessage) (api.ShellResult, error) {
	var args protocol.ShellWhichArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.ShellResult{}, err
	}
	if strings.TrimSpace(args.Command) == "" {
		return api.ShellResult{}, invalid("a command is required")
	}
	result, err := h.run(ctx, platform.CommandSpec{Binary: whichBinary, Args: []string{args.Command}}, api.ShellOptions{})
	result.Stdout = strings.TrimSpace(result.Stdout)
	return result, err
}

// run starts one process under the call's deadline and reports what it printed. A process killed at the deadline
// is a result with exit code -1, as the Electron host reports it; a program that cannot start is a result with exit
// code 1. Only the call's own cancellation is an error.
func (h *Handler) run(ctx context.Context, spec platform.CommandSpec, options api.ShellOptions) (api.ShellResult, error) {
	spec.Dir = h.expandHome(options.CWD)
	spec.Env = options.Env
	timeout := processTimeout(options.TimeoutMs)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	limit := outputLimit(options.OutputLimit)
	if err := h.admitDesktopEffect(ctx); err != nil {
		return api.ShellResult{}, err
	}
	result, err := h.deps.Commands.Run(runCtx, spec)
	stdout := limitOutput(result.Stdout, limit, result.Truncated)
	stderr := limitOutput(result.Stderr, limit, result.Truncated)
	switch {
	case ctx.Err() != nil:
		return api.ShellResult{}, ctx.Err()
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		return api.ShellResult{Stdout: stdout, Stderr: stderr + fmt.Sprintf("\nKilled after %dms", timeout.Milliseconds()), ExitCode: -1}, nil
	case err != nil:
		return api.ShellResult{Stderr: err.Error(), ExitCode: 1}, nil
	}
	return api.ShellResult{Stdout: stdout, Stderr: stderr, ExitCode: result.ExitCode}, nil
}

// expandHome replaces a leading "~" with the user's home folder. Without a home folder the value is returned as is.
func (h *Handler) expandHome(value string) string {
	if h.deps.Home == "" || !(value == "~" || strings.HasPrefix(value, "~/")) {
		return value
	}
	return h.deps.Home + value[1:]
}

// processTimeout clamps the requested timeout to the range the Electron host allows. Zero takes the default.
func processTimeout(milliseconds int64) time.Duration {
	timeout := time.Duration(milliseconds) * time.Millisecond
	switch {
	case milliseconds <= 0:
		return defaultProcessTimeout
	case timeout < minProcessTimeout:
		return minProcessTimeout
	case timeout > maxProcessTimeout:
		return maxProcessTimeout
	}
	return timeout
}

// outputLimit returns the characters kept per stream. Zero takes the default, and nothing exceeds the port's cap.
func outputLimit(requested int64) int {
	switch {
	case requested <= 0:
		return defaultOutputLimit
	case requested > platform.MaxHelperOutputBytes:
		return platform.MaxHelperOutputBytes
	}
	return int(requested)
}

// limitOutput keeps the first limit bytes of one stream and marks it when bytes were cut. portTruncated is the
// port's own cap: when it reports a cut and the stream reached the limit, the stream was cut too.
func limitOutput(output []byte, limit int, portTruncated bool) string {
	cut := len(output) > limit || (portTruncated && len(output) >= limit)
	if len(output) > limit {
		output = output[:limit]
	}
	text := strings.ToValidUTF8(string(output), "")
	if cut {
		text += truncatedOutputMarker
	}
	return text
}
