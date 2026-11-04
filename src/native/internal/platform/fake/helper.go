package fake

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// ErrNoHelperScript is returned by Helper.Run when a binary has no queued script, so an unexpected child
// process fails the test instead of running for real.
var ErrNoHelperScript = errors.New("fake helper: no script queued for this binary")

// HelperScript is one scripted run. A Timeout script returns context.DeadlineExceeded at once, as a helper
// killed at its deadline would, without waiting. Err is returned as the run's error instead of a result.
type HelperScript struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Timeout  bool
	Err      error
}

// HelperCall is one recorded run.
type HelperCall struct {
	Binary string
	Args   []string
	Stdin  []byte
}

// Helper answers Run from per-binary queues of scripts and records every call. Output is capped at
// platform.MaxHelperOutputBytes exactly as the real helper caps it.
//
//	helper := fake.NewHelper()
//	helper.Script("/usr/bin/xdotool", fake.HelperScript{ExitCode: 0})
type Helper struct {
	mu      sync.Mutex
	scripts map[string][]HelperScript
	calls   []HelperCall
}

var _ platform.Helper = (*Helper)(nil)

// NewHelper returns a helper with no scripts.
func NewHelper() *Helper {
	return &Helper{scripts: map[string][]HelperScript{}}
}

// Script queues runs for binary, used in order.
func (h *Helper) Script(binary string, scripts ...HelperScript) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.scripts[binary] = append(h.scripts[binary], scripts...)
}

// Calls returns the recorded runs in order.
func (h *Helper) Calls() []HelperCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.calls)
}

// Run records the call and returns the next script for binary. A context that is already done fails first,
// as the real helper does.
func (h *Helper) Run(ctx context.Context, binary string, args []string, stdin []byte) (platform.HelperResult, error) {
	h.record(binary, args, stdin)
	if err := ctx.Err(); err != nil {
		return platform.HelperResult{}, err
	}
	script, ok := h.nextScript(binary)
	if !ok {
		return platform.HelperResult{}, fmt.Errorf("%w: %s", ErrNoHelperScript, binary)
	}
	if script.Timeout {
		return platform.HelperResult{}, context.DeadlineExceeded
	}
	if script.Err != nil {
		return platform.HelperResult{}, script.Err
	}
	stdout, stdoutTruncated := capOutput(script.Stdout)
	stderr, stderrTruncated := capOutput(script.Stderr)
	return platform.HelperResult{
		Stdout:    stdout,
		Stderr:    stderr,
		ExitCode:  script.ExitCode,
		Truncated: stdoutTruncated || stderrTruncated,
	}, nil
}

// record stores a copy of the call.
func (h *Helper) record(binary string, args []string, stdin []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var copiedStdin []byte
	if stdin != nil {
		copiedStdin = bytes.Clone(stdin)
	}
	h.calls = append(h.calls, HelperCall{Binary: binary, Args: slices.Clone(args), Stdin: copiedStdin})
}

// nextScript pops the next script queued for binary.
func (h *Helper) nextScript(binary string) (HelperScript, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	queue := h.scripts[binary]
	if len(queue) == 0 {
		return HelperScript{}, false
	}
	h.scripts[binary] = queue[1:]
	return queue[0], true
}

// capOutput truncates output to the platform limit and reports whether it did.
func capOutput(output []byte) ([]byte, bool) {
	if len(output) <= platform.MaxHelperOutputBytes {
		return bytes.Clone(output), false
	}
	return bytes.Clone(output[:platform.MaxHelperOutputBytes]), true
}
