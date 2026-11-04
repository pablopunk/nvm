package jobs

import (
	"context"
	"errors"
	"time"
)

// Owner says who declared a job. Host jobs ship with the app; extension jobs come from an extension's commands.
type Owner string

const (
	OwnerHost      Owner = "host"
	OwnerExtension Owner = "extension"
)

// TriggerKind is how a job is started.
type TriggerKind string

const (
	// TriggerStartup runs the job once, Delay after it is registered.
	TriggerStartup TriggerKind = "startup"
	// TriggerInterval runs the job every Every, first after Delay (or Every when Delay is zero).
	TriggerInterval TriggerKind = "interval"
	// TriggerEvent runs the job when Scheduler.Emit names Event, after Debounce of quiet, at most MaxWait late.
	TriggerEvent TriggerKind = "event"
)

// Events the scheduler emits itself. Extensions and host jobs may subscribe to them like any other event.
const (
	EventWake  = "wake"
	EventLogin = "login"
)

// MissedPolicy says what an interval job does at registration when a run fell due while the app was closed.
type MissedPolicy string

const (
	// MissedCatchUpOnce runs the job once, at the first trigger delay, and never more than once for the gap.
	MissedCatchUpOnce MissedPolicy = "catch-up-once"
	// MissedSkip waits for the next regular run.
	MissedSkip MissedPolicy = "skip"
)

// MinInterval floors interval triggers so that a typo cannot schedule a busy loop.
const MinInterval = time.Second

// Trigger declares one way a job starts. Fields not used by its Kind must stay zero.
type Trigger struct {
	Kind     TriggerKind   `json:"kind"`
	Delay    time.Duration `json:"delay,omitempty"`
	Every    time.Duration `json:"every,omitempty"`
	Missed   MissedPolicy  `json:"missed,omitempty"`
	Event    string        `json:"event,omitempty"`
	Debounce time.Duration `json:"debounce,omitempty"`
	MaxWait  time.Duration `json:"maxWait,omitempty"`
}

// Budget bounds one job: how long a run may take and how many runs may start in a window.
type Budget struct {
	// Timeout is the longest a run may take before its context is cancelled. Zero means DefaultTimeout.
	Timeout time.Duration
	// MaxRuns is how many runs may start within Window. Zero means no limit; a run over the limit is skipped and
	// recorded in the history, and it is not a failure.
	MaxRuns int
	Window  time.Duration
}

// Backoff spaces the runs after a failure: the first failure waits Initial, each further one doubles it, up to Max.
// Zero fields mean DefaultBackoffInitial and DefaultBackoffMax.
type Backoff struct {
	Initial time.Duration
	Max     time.Duration
}

// HaltPolicy stops a job after Failures failed runs within Within, until the user enables it again. Zero fields mean
// DefaultHaltFailures within DefaultHaltWindow.
type HaltPolicy struct {
	Failures int
	Within   time.Duration
}

// Defaults for the fields a definition leaves at zero.
const (
	DefaultTimeout        = time.Minute
	DefaultBackoffInitial = 30 * time.Second
	DefaultBackoffMax     = 15 * time.Minute
	DefaultHaltFailures   = 3
	DefaultHaltWindow     = time.Minute
	maxHistory            = 20
)

// Payload is the data an event carries into a run. changedPaths entries merge across debounced events.
type Payload map[string]any

// Run is what a job's function receives: why it runs, the merged event payload and when it started.
type Run struct {
	ID        string
	Reason    string
	Payload   Payload
	StartedAt time.Time
}

// Definition declares one job. Register replaces a job with the same ID and keeps its persisted state.
type Definition struct {
	ID    string
	Title string
	Owner Owner
	Scope string
	// StartDisabled keeps the job off until the user enables it. Enabled jobs run their triggers.
	StartDisabled bool
	Triggers      []Trigger
	Budget        Budget
	Backoff       Backoff
	Halt          HaltPolicy
	// Run does the work. It must return when ctx is done; a run that ignores ctx holds the job until it returns.
	Run func(ctx context.Context, run Run) error
}

// Status is the state a job shows in the palette.
type Status string

const (
	StatusIdle       Status = "idle"
	StatusRunning    Status = "running"
	StatusSucceeded  Status = "succeeded"
	StatusFailed     Status = "failed"
	StatusBackingOff Status = "backing-off"
	StatusDisabled   Status = "disabled"
)

// Outcome is how one run ended.
type Outcome string

const (
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
	// OutcomeSkipped is a run the budget refused. It never ran and is not a failure.
	OutcomeSkipped Outcome = "skipped"
)

// HistoryEntry is one finished or refused run, newest first in Snapshot.History.
type HistoryEntry struct {
	StartedAt  time.Time     `json:"startedAt"`
	FinishedAt time.Time     `json:"finishedAt"`
	Duration   time.Duration `json:"duration"`
	Outcome    Outcome       `json:"outcome"`
	Reason     string        `json:"reason"`
	Error      string        `json:"error,omitempty"`
}

// Snapshot is a copy of one job's state for views.
type Snapshot struct {
	ID           string
	Title        string
	Owner        Owner
	Scope        string
	Status       Status
	Enabled      bool
	Halted       string
	Running      bool
	Triggers     []Trigger
	Runs         int
	Failures     int
	Consecutive  int
	LastReason   string
	LastStarted  time.Time
	LastFinished time.Time
	LastDuration time.Duration
	LastError    string
	NextRunAt    time.Time
	BackoffUntil time.Time
	History      []HistoryEntry
}

// Errors returned by the scheduler's controls.
var (
	ErrUnknownJob        = errors.New("jobs: no job with that id")
	ErrDisabled          = errors.New("jobs: the job is disabled")
	ErrClosed            = errors.New("jobs: the scheduler is closed")
	ErrInvalidDefinition = errors.New("jobs: invalid definition")
)
