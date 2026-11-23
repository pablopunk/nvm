package api

// UpdateState is a snapshot of the host update manager. Empty strings mean "none". Message is the one line the person
// reads about the update, the same line the menu bar and the root show.
type UpdateState struct {
	CurrentVersion    string `json:"currentVersion"`
	Channel           string `json:"channel"` // one of the UpdateChannel values
	Status            string `json:"status"`  // one of the UpdateStatus values
	Supported         bool   `json:"supported"`
	Checking          bool   `json:"checking"`
	Downloading       bool   `json:"downloading"`
	Installing        bool   `json:"installing"`
	AvailableVersion  string `json:"availableVersion,omitempty"`
	DownloadedVersion string `json:"downloadedVersion,omitempty"`
	ErrorMessage      string `json:"errorMessage,omitempty"`
	// FailedStage is the UpdateStage an error status failed at.
	FailedStage string `json:"failedStage,omitempty"`
	Message     string `json:"message,omitempty"`
	// CheckedAtMs is when the last check on this channel finished; 0 before the first.
	CheckedAtMs int64 `json:"checkedAtMs,omitempty"`
}

// Status values of UpdateState.Status, as the host's update manager reports them.
const (
	UpdateStatusIdle        = "idle"
	UpdateStatusCurrent     = "current"
	UpdateStatusUnsupported = "unsupported"
	UpdateStatusChecking    = "checking"
	UpdateStatusAvailable   = "available"
	UpdateStatusDownloading = "downloading"
	UpdateStatusDownloaded  = "downloaded"
	UpdateStatusInstalling  = "installing"
	UpdateStatusError       = "error"
)

// Stage values of UpdateState.FailedStage.
const (
	UpdateStageCheck   = "check"
	UpdateStageInstall = "install"
)

// Channel values of UpdateState.Channel. Stable offers final releases only; nightly and dev also offer prereleases,
// and a final release still wins whenever it is newer.
const (
	UpdateChannelStable  = "stable"
	UpdateChannelNightly = "nightly"
	UpdateChannelDev     = "dev"
)

// Operations of an ActionUpdate action. Check looks for a newer version, download starts one that waits, install
// restarts into a downloaded one, and channel moves to the channel in Target (the next one without it), checks it and,
// with a Target, goes back a page.
const (
	UpdateOpCheck    = "check"
	UpdateOpDownload = "download"
	UpdateOpInstall  = "install"
	UpdateOpChannel  = "channel"
)

// Updates reads update state. Check, download, install and channel are ActionUpdate actions, and the
// host performs them and renders their progress.
type Updates interface {
	State() UpdateState
}
