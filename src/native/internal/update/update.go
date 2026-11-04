package update

import (
	"context"
	"strconv"
)

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

// Stage names the step that failed when the status is StatusFailed, so the message says what went wrong.
type Stage string

// The steps that can fail. The engine swaps the app's files only after the whole update is downloaded and verified,
// so a failed install leaves the installed app as it was.
const (
	StageCheck   Stage = "check"
	StageInstall Stage = "install"
)

// State is what an updater knows: its status, the running version, the newest version it has seen, the download
// fraction while one runs, the failed step, and a message for the person when the status needs one.
type State struct {
	Status         Status
	CurrentVersion string
	LatestVersion  string
	Progress       float64
	Stage          Stage
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

// NoUpdates is the updater of builds that cannot update themselves. It never contacts the network.
type NoUpdates struct {
	Version string
}

// unsupportedMessage is what the person reads when a check is asked of a build that cannot update.
const unsupportedMessage = "Updates are not available in this build yet"

// State reports the unsupported status with the running version.
func (u NoUpdates) State() State {
	return unsupportedState(u.Version)
}

// Check answers like State: there is nothing to look for.
func (u NoUpdates) Check(context.Context) State {
	return u.State()
}

func unsupportedState(current string) State {
	return State{Status: StatusUnsupported, CurrentVersion: current, Message: unsupportedMessage}
}

// Message is the one line the person reads after a check or while an update moves. An available version is named,
// and a failure says which step failed and why.
func Message(s State) string {
	switch s.Status {
	case StatusAvailable:
		return "Nevermind " + s.LatestVersion + " is available"
	case StatusDownloading:
		return "Downloading Nevermind " + s.LatestVersion + " (" + strconv.Itoa(percent(s.Progress)) + "%)"
	case StatusDownloaded:
		return "Nevermind " + s.LatestVersion + " is ready. Restart to finish updating."
	case StatusInstalling:
		return "Restarting to finish updating Nevermind " + s.LatestVersion
	case StatusChecking:
		return "Checking for updates"
	case StatusFailed:
		return failurePrefix(s.Stage) + s.Message
	case StatusUnsupported:
		return unsupportedMessage
	default:
		return "Nevermind is up to date"
	}
}

func failurePrefix(stage Stage) string {
	if stage == StageInstall {
		return "Update install failed: "
	}
	return "Update check failed: "
}

func percent(fraction float64) int {
	return int(fraction*100 + 0.5)
}
