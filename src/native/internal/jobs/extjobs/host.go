package extjobs

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/jobs"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Host is the part of the extension manager the adapter uses. *manager.Manager satisfies it.
type Host interface {
	Commands(ctx context.Context) []manager.CommandResult
	Run(ctx context.Context, id string, invocation api.Invocation) (api.Result, *api.Failure, error)
}

// Registry is the part of the scheduler the adapter uses. *jobs.Scheduler satisfies it.
type Registry interface {
	Register(def jobs.Definition) error
	Unregister(id string)
	Snapshot() []jobs.Snapshot
	Emit(event string, payload jobs.Payload)
}

// Names and timings of the host job that keeps the extension jobs in step with the running extensions. The
// interval is a safety net for an extension that finished starting after the last sync.
const (
	SyncJobID    = "extensions.triggers"
	SyncEvent    = "extensions.changed"
	SyncDelay    = 2 * time.Second
	SyncEvery    = 15 * time.Second
	syncDebounce = 300 * time.Millisecond
	syncTimeout  = 30 * time.Second
)

// Event names the host emits for the trigger types that do not come from a folder. The app emits clipboard.changed.
const (
	ClipboardChangedEvent = "clipboard.changed"
	FrontmostChangedEvent = "app.frontmost.changed"
)

// JobPrefix starts the ID of every job an extension command owns: "extension.<extension id>.<command id>".
const JobPrefix = "extension."

// Config wires the adapter. Host returns nil while the extensions are still loading, and the sync job does nothing
// then. Files watches the roots of files.changed triggers; Home expands a leading "~" in them.
type Config struct {
	Host     func() Host
	Registry Registry
	Files    platform.FS
	Home     string
	// Log receives the diagnostics of triggers the host cannot run. Nil means the process logger.
	Log *logx.Logger
}

// jobIDFor is the scheduler ID of one command of one extension.
func jobIDFor(extensionID, commandID string) string {
	return JobPrefix + extensionID + "." + commandID
}
