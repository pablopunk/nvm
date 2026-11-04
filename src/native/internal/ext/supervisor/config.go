// Package supervisor runs one extension in a child process and keeps the palette safe from it. The host talks to
// the child over the line protocol (package protocol) and enforces the call budgets (B8: Search 20 ms soft,
// 100 ms hard). A call that overruns its hard budget, a child that crashes, and a child that exceeds its memory
// limit are all failures: the child is killed, the failure is recorded with a reason, and the next call restarts
// it after a backoff. Repeated failures disable the extension until someone resets it.
//
// Layer: ext. The package imports api, protocol and platform (ports) and the standard library only.
package supervisor

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Defaults for a Config whose fields are zero.
const (
	DefaultStartTimeout = 2 * time.Second
	DefaultMaxFailures  = 3
	DefaultFailureWin   = time.Minute
	backoffBase         = 250 * time.Millisecond
	backoffMax          = 30 * time.Second
	memoryWatchInterval = time.Second
	maxDiagnostics      = 20
)

// Budget is the time one call may take. Soft is recorded as a diagnostic; Hard kills the child. A zero Soft
// records nothing; a zero Hard is not allowed.
type Budget struct {
	Soft time.Duration
	Hard time.Duration
}

// DefaultBudgets are the B8 budgets for the calls the palette makes on every keystroke, and a looser budget for
// the rest.
var DefaultBudgets = map[string]Budget{
	protocol.MethodSearch:   {Soft: 20 * time.Millisecond, Hard: 100 * time.Millisecond},
	protocol.MethodRoot:     {Soft: 20 * time.Millisecond, Hard: 100 * time.Millisecond},
	protocol.MethodCommands: {Soft: 20 * time.Millisecond, Hard: time.Second},
	protocol.MethodRun:      {Hard: 5 * time.Second},
	protocol.MethodLoad:     {Hard: 10 * time.Second},

	protocol.MethodMenuBars:        {Soft: 50 * time.Millisecond, Hard: time.Second},
	protocol.MethodMenuBarTriggers: {Soft: 20 * time.Millisecond, Hard: 500 * time.Millisecond},
}

// Config is everything a Runtime needs from the composition root. Ports are injected so tests can fake them.
type Config struct {
	Worker platform.Worker
	Clock  platform.Clock
	// Processes lets the resident memory watch read the child's size. Nil disables the watch.
	Processes platform.Processes
	// Executable is this binary. The child is started from it with protocol.ChildFlag.
	Executable string
	// MemoryLimit caps the child's address space, in bytes. Zero means no cap. Linux enforces it.
	MemoryLimit uint64
	// ResidentLimit is the resident memory above which the host kills the child, in bytes. Zero disables it.
	ResidentLimit uint64
	StartTimeout  time.Duration
	Budgets       map[string]Budget
	MaxFailures   int
	FailureWindow time.Duration
}

func (c Config) startTimeout() time.Duration {
	if c.StartTimeout > 0 {
		return c.StartTimeout
	}
	return DefaultStartTimeout
}

func (c Config) maxFailures() int {
	if c.MaxFailures > 0 {
		return c.MaxFailures
	}
	return DefaultMaxFailures
}

func (c Config) failureWindow() time.Duration {
	if c.FailureWindow > 0 {
		return c.FailureWindow
	}
	return DefaultFailureWin
}

// BudgetFor returns the budget of a method: the configured one, else the default one, else a hard second.
func (c Config) BudgetFor(method string) Budget {
	if budget, ok := c.Budgets[method]; ok && budget.Hard > 0 {
		return budget
	}
	if budget, ok := DefaultBudgets[method]; ok {
		return budget
	}
	return Budget{Hard: time.Second}
}

// State is where an extension's child process is in its life.
type State string

const (
	// StateIdle means the extension is not started. Start moves it on.
	StateIdle State = "idle"
	// StateStarting means a child was launched and has not reported ready yet.
	StateStarting State = "starting"
	// StateRunning means a child is ready and answers calls.
	StateRunning State = "running"
	// StateFailed means the last child failed. Calls fail fast until the backoff passes, then the next call
	// starts a new child.
	StateFailed State = "failed"
	// StateBroken means the child reported that the source does not load. It is not retried until the source
	// changes, which the owner signals by stopping and starting again.
	StateBroken State = "broken"
	// StateDisabled means the extension failed too often inside the failure window. Only Reset clears it.
	StateDisabled State = "disabled"
)

// Status is a snapshot for the manager and the palette's error surfaces.
type Status struct {
	State       State
	Reason      string
	Failures    int
	Diagnostics []string
}
