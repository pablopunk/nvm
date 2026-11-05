package updates

import "github.com/pablopunk/nvm/native/internal/ext/api"

// Ids of the rows.
const (
	rowStatus  = "update-status"
	rowRestart = "update-restart"
	rowCheck   = "update-check"
	rowChannel = "update-channel"
)

// checksEvery is the schedule the status line states; the updater checks on it.
const checksEvery = "Checks every 4 hours"

// updateRows are the rows of the page: the status line, a restart when an update is downloaded, the check and the
// channel switch.
func updateRows(state api.UpdateState) []api.Item {
	rows := []api.Item{statusRow(state)}
	if state.Status == api.UpdateStatusDownloaded {
		rows = append(rows, restartRow(state))
	}
	return append(rows, checkRow(), channelRow(state))
}

// statusRow says where the updater stands in the words the updater reports, and the channel and schedule under it.
func statusRow(state api.UpdateState) api.Item {
	return api.Item{
		ID: rowStatus, Title: state.Message, Subtitle: statusSubtitle(state), Icon: statusIcon(state.Status),
		Contextual: state.Status == api.UpdateStatusError, Disabled: true,
	}
}

// restartRow installs the downloaded update, which restarts the app; the palette closes as the restart begins.
func restartRow(state api.UpdateState) api.Item {
	return api.Item{
		ID: rowRestart, Title: "Restart to Update", Subtitle: "Nevermind " + state.DownloadedVersion + " is installed; restart to run it",
		Icon: "rotate-ccw", Appearance: &api.Appearance{Foreground: api.ColorNone, Prominence: api.ProminencePrimary},
		Actions: []api.Action{{Kind: api.ActionUpdate, Op: api.UpdateOpInstall, Title: "Restart", DismissAfterRun: true}},
	}
}

// checkRow looks for a newer version now, and the page shows the answer as its status line.
func checkRow() api.Item {
	return api.Item{
		ID: rowCheck, Title: "Check for Updates", Subtitle: "Look for a newer version now", Icon: "rotate-ccw",
		Actions: []api.Action{{Kind: api.ActionUpdate, Op: api.UpdateOpCheck, Title: "Check"}},
	}
}

// channelRow moves to the next release channel, Stable, Nightly, Dev and back, and checks it at once.
func channelRow(state api.UpdateState) api.Item {
	return api.Item{
		ID: rowChannel, Title: "Update Channel", Subtitle: channelSubtitle(state), Icon: "shield-check",
		Accessories: []api.Accessory{{Text: channelLabel(state.Channel)}},
		Actions:     []api.Action{{Kind: api.ActionUpdate, Op: api.UpdateOpChannel, Title: "Switch"}},
	}
}

// statusSubtitle names the channel and the schedule, but only in a build that checks: an unsupported build checks nothing.
func statusSubtitle(state api.UpdateState) string {
	if state.Status == api.UpdateStatusUnsupported {
		return ""
	}
	return channelLabel(state.Channel) + " channel · " + checksEvery
}

// statusIcon is the icon of the status line for a status.
func statusIcon(status string) string {
	switch status {
	case api.UpdateStatusError:
		return "circle-alert"
	case api.UpdateStatusAvailable, api.UpdateStatusDownloading:
		return "arrow-down"
	case api.UpdateStatusDownloaded, api.UpdateStatusInstalling:
		return "rotate-ccw"
	}
	return "check"
}

// channelLabel is the name a channel shows as.
func channelLabel(channel string) string {
	switch channel {
	case api.UpdateChannelNightly:
		return "Nightly"
	case api.UpdateChannelDev:
		return "Dev"
	}
	return "Stable"
}

// channelSubtitle says what a channel offers. An unsupported build has no channel to choose.
func channelSubtitle(state api.UpdateState) string {
	switch {
	case state.Status == api.UpdateStatusUnsupported:
		return "Not available in this build yet"
	case state.Channel == api.UpdateChannelNightly:
		return "Includes nightly prereleases; a final release still wins when newer"
	case state.Channel == api.UpdateChannelDev:
		return "Includes frequent dev builds and nightly prereleases; a final release still wins when newer"
	}
	return "Final releases only"
}
