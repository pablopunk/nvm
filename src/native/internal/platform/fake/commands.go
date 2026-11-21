package fake

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// CommandsCall is one recorded process run, with the environment it was given.
type CommandsCall struct {
	Binary string
	Args   []string
	Dir    string
	Env    map[string]string
}

// Commands answers Run from per-binary queues of scripts, as Helper does, and records every call. A script with
// Timeout waits for ctx to end, as a process killed at its deadline does.
type Commands struct {
	mu      sync.Mutex
	scripts map[string][]HelperScript
	calls   []CommandsCall
}

var _ platform.Commands = (*Commands)(nil)

// NewCommands returns a command runner with no scripts.
func NewCommands() *Commands {
	return &Commands{scripts: map[string][]HelperScript{}}
}

// Script queues runs for binary, used in order.
func (c *Commands) Script(binary string, scripts ...HelperScript) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scripts[binary] = append(c.scripts[binary], scripts...)
}

// Calls returns the recorded runs in order.
func (c *Commands) Calls() []CommandsCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.calls)
}

// Run records the call and answers it with the next script queued for the binary.
func (c *Commands) Run(ctx context.Context, spec platform.CommandSpec) (platform.HelperResult, error) {
	c.record(spec)
	script, ok := c.nextScript(spec.Binary)
	if !ok {
		return platform.HelperResult{}, fmt.Errorf("%w: %s", ErrNoHelperScript, spec.Binary)
	}
	if script.Timeout {
		<-ctx.Done()
		return platform.HelperResult{}, ctx.Err()
	}
	if script.Err != nil {
		return platform.HelperResult{}, script.Err
	}
	stdout, stdoutTruncated := capOutput(script.Stdout)
	stderr, stderrTruncated := capOutput(script.Stderr)
	return platform.HelperResult{Stdout: stdout, Stderr: stderr, ExitCode: script.ExitCode, Truncated: stdoutTruncated || stderrTruncated}, nil
}

func (c *Commands) record(spec platform.CommandSpec) {
	c.mu.Lock()
	defer c.mu.Unlock()
	env := map[string]string{}
	for key, value := range spec.Env {
		env[key] = value
	}
	c.calls = append(c.calls, CommandsCall{Binary: spec.Binary, Args: slices.Clone(spec.Args), Dir: spec.Dir, Env: env})
}

func (c *Commands) nextScript(binary string) (HelperScript, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	queue := c.scripts[binary]
	if len(queue) == 0 {
		return HelperScript{}, false
	}
	c.scripts[binary] = queue[1:]
	return queue[0], true
}
