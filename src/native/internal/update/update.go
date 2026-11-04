package update

import "context"

// Status is the updater's state. The names follow the 1.x update manager, so the palette and the menu read the same.
type Status string

// The statuses an updater reports.
const (
	StatusIdle        Status = "idle"
	StatusUnsupported Status = "unsupported"
	StatusChecking    Status = "checking"
	StatusAvailable   Status = "available"
	StatusDownloading Status = "downloading"
	StatusDownloaded  Status = "downloaded"
	StatusInstalling  Status = "installing"
	StatusFailed      Status = "error"
)

// State is what an updater knows: its status, the running version, the newest version it has seen, and a message
// for the person when the status needs one.
type State struct {
	Status         Status
	CurrentVersion string
	LatestVersion  string
	Message        string
}

// Updater is the port the menu and the palette use. Check may take a while and must honor ctx; callers run it off
// the UI thread.
type Updater interface {
	// State returns what the updater knows without contacting anything.
	State() State
	// Check looks for a newer version and returns the state after the check.
	Check(ctx context.Context) State
}

// NoUpdates is the updater of builds that cannot update themselves yet. It never contacts the network.
type NoUpdates struct {
	Version string
}

// unsupportedMessage is what the person reads when a check is asked of a build that cannot update.
const unsupportedMessage = "Updates are not available in this build yet"

// State reports the unsupported status with the running version.
func (u NoUpdates) State() State {
	return State{Status: StatusUnsupported, CurrentVersion: u.Version, Message: unsupportedMessage}
}

// Check answers like State: there is nothing to look for.
func (u NoUpdates) Check(context.Context) State {
	return u.State()
}

// Message is the one line the person reads after a check. An available version is named, and a failure says why.
func Message(s State) string {
	switch s.Status {
	case StatusAvailable, StatusDownloading, StatusDownloaded, StatusInstalling:
		return "Nevermind " + s.LatestVersion + " is available"
	case StatusFailed:
		return "Update check failed: " + s.Message
	case StatusUnsupported:
		return unsupportedMessage
	default:
		return "Nevermind is up to date"
	}
}
