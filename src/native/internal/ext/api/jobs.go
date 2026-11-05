package api

import "time"

// Owners of a job: the host runs its own jobs, and an extension runs the ones it declared.
const (
	JobOwnerHost      = "host"
	JobOwnerExtension = "extension"
)

// Statuses of a job, as the host's scheduler reports them. Halted jobs read as disabled with a Halted reason.
const (
	JobStatusIdle       = "idle"
	JobStatusRunning    = "running"
	JobStatusSucceeded  = "succeeded"
	JobStatusFailed     = "failed"
	JobStatusBackingOff = "backing-off"
	JobStatusDisabled   = "disabled"
)

// Kinds of a job trigger. A job with no trigger runs only when someone runs it.
const (
	JobTriggerStartup  = "startup"
	JobTriggerInterval = "interval"
	JobTriggerEvent    = "event"
)

// Outcomes of one run of a job.
const (
	JobOutcomeSucceeded = "succeeded"
	JobOutcomeFailed    = "failed"
	JobOutcomeSkipped   = "skipped"
)

// JobTrigger is one thing that starts a job: at startup, every EveryMs milliseconds, or on a host event.
type JobTrigger struct {
	Kind    string `json:"kind"`
	EveryMs int64  `json:"everyMs,omitempty"`
	Event   string `json:"event,omitempty"`
}

// JobRun is one finished run of a job, in the order the history keeps them.
type JobRun struct {
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	DurationMs int64     `json:"durationMs"`
	Outcome    string    `json:"outcome"`
	Reason     string    `json:"reason"`
	Error      string    `json:"error,omitempty"`
}

// Job is one background job the host schedules: its state, its triggers and its recent runs. The host's own jobs and the
// jobs extensions declared are listed together, each with its owner.
type Job struct {
	ID                  string       `json:"id"`
	Title               string       `json:"title"`
	Owner               string       `json:"owner"`
	Scope               string       `json:"scope,omitempty"`
	Status              string       `json:"status"`
	Enabled             bool         `json:"enabled"`
	Halted              string       `json:"halted,omitempty"`
	Running             bool         `json:"running"`
	Triggers            []JobTrigger `json:"triggers,omitempty"`
	Runs                int          `json:"runs"`
	Failures            int          `json:"failures"`
	ConsecutiveFailures int          `json:"consecutiveFailures"`
	LastReason          string       `json:"lastReason,omitempty"`
	LastStartedAt       time.Time    `json:"lastStartedAt"`
	LastFinishedAt      time.Time    `json:"lastFinishedAt"`
	LastDurationMs      int64        `json:"lastDurationMs"`
	LastError           string       `json:"lastError,omitempty"`
	NextRunAt           time.Time    `json:"nextRunAt"`
	BackoffUntil        time.Time    `json:"backoffUntil"`
	History             []JobRun     `json:"history,omitempty"`
}

// Jobs lists the background jobs and changes them. Listing needs no declaration. Running a job, enabling or disabling it,
// and clearing its error change the host's work, so they need the jobs declaration. Each returns the reason it refused.
type Jobs interface {
	List() []Job
	Run(id string) error
	SetEnabled(id string, enabled bool) error
	ClearError(id string) error
}
