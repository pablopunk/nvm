package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/update"
)

// updateStatus is what the updater reports at one moment: its state and the channel its checks follow.
type updateStatus struct {
	State   update.State
	Channel update.Channel
}

// updateSnapshot is the updater's status. Without an updater it is unsupported on stable.
func (p *App) updateSnapshot() updateStatus {
	if p.updates == nil {
		return updateStatus{State: update.NoUpdates{}.State(), Channel: update.ChannelStable}
	}
	return updateStatus{State: p.updates.State(), Channel: p.updates.Channel()}
}

// updatesSurface serves the updater to the extension API: the state the Updates built-in reads.
type updatesSurface struct {
	p *App
}

// State is the updater's status in the extension API's shape.
func (s updatesSurface) State() api.UpdateState {
	return apiUpdateState(s.p.updateSnapshot())
}

// apiUpdateState converts the updater's status for the extension API. Message is the line the menu bar and the root
// show for the same state, so every surface says the same thing.
func apiUpdateState(status updateStatus) api.UpdateState {
	state := status.State
	return api.UpdateState{
		CurrentVersion:    state.CurrentVersion,
		Channel:           string(status.Channel),
		Status:            string(state.Status),
		Supported:         state.Status != update.StatusUnsupported,
		Checking:          state.Status == update.StatusChecking,
		Downloading:       state.Status == update.StatusDownloading,
		Installing:        state.Status == update.StatusInstalling,
		AvailableVersion:  availableVersion(state),
		DownloadedVersion: downloadedVersion(state),
		ErrorMessage:      failureMessage(state),
		Message:           update.Message(state),
	}
}

// availableVersion is the newer version the updater has found, from the check until the update is installed.
func availableVersion(state update.State) string {
	switch state.Status {
	case update.StatusAvailable, update.StatusDownloading, update.StatusDownloaded, update.StatusInstalling:
		return state.LatestVersion
	}
	return ""
}

// downloadedVersion is the newer version that is downloaded and waits for its restart.
func downloadedVersion(state update.State) string {
	switch state.Status {
	case update.StatusDownloaded, update.StatusInstalling:
		return state.LatestVersion
	}
	return ""
}

// failureMessage is the cause of a failed check or install, and nothing when the updater did not fail.
func failureMessage(state update.State) string {
	if state.Status != update.StatusFailed {
		return ""
	}
	return state.Message
}
