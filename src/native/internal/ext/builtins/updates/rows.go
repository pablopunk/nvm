package updates

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// Ids of the page's rows.
const (
	rowStatus  = "update-status"
	rowChannel = "update-channel"
)

// checksEvery is the schedule the status line states before the first check; the updater checks on it.
const checksEvery = "Checks every 4 hours"

// updateRows are the rows of the page: the status row, which is also the check and the restart, and the channel.
func updateRows(state api.UpdateState, now time.Time) []api.Item {
	return []api.Item{statusRow(state, now), channelRow(state)}
}

// statusRow says where the updater stands, and Enter on it does the next thing that state allows: check, try again or
// restart. While a check, a download or a restart runs, the row spins and has no action, so Enter cannot start another.
func statusRow(state api.UpdateState, now time.Time) api.Item {
	row := api.Item{ID: rowStatus, Title: state.Message, Subtitle: checkedSubtitle(state, now), Icon: "check"}
	switch state.Status {
	case api.UpdateStatusUnsupported:
		row.Icon, row.Subtitle, row.Disabled = "info", "", true
	case api.UpdateStatusIdle:
		row.Title, row.Subtitle, row.Icon = "Check for Updates", "Not checked yet · "+channelLabel(state.Channel)+" channel", "rotate-ccw"
		row.Actions = []api.Action{checkAction("Check")}
	case api.UpdateStatusCurrent:
		row.Actions = []api.Action{checkAction("Check Again")}
	case api.UpdateStatusChecking:
		row.Title, row.Subtitle, row.Busy = "Checking for updates…", "Looking for a newer version", true
	case api.UpdateStatusAvailable, api.UpdateStatusDownloading:
		row.Subtitle, row.Icon, row.Busy = "Installs in the background; keep working", "arrow-down", true
	case api.UpdateStatusDownloaded:
		return restartRow(state)
	case api.UpdateStatusInstalling:
		row.Subtitle, row.Icon, row.Busy = "Nevermind opens again in a moment", "rotate-ccw", true
	case api.UpdateStatusError:
		row.Title, row.Subtitle, row.Icon = failureTitle(state), state.ErrorMessage, "circle-alert"
		row.Actions = []api.Action{checkAction("Try Again")}
	}
	return row
}

// restartRow is the status row once an update is downloaded: Enter restarts into it, and the palette closes as the
// restart begins.
func restartRow(state api.UpdateState) api.Item {
	return api.Item{
		ID: rowStatus, Title: "Restart to Update", Subtitle: "Nevermind " + state.DownloadedVersion + " is ready",
		Icon: "rotate-ccw", Appearance: &api.Appearance{Foreground: api.ColorNone, Prominence: api.ProminencePrimary},
		Actions: []api.Action{{Kind: api.ActionUpdate, Op: api.UpdateOpInstall, Title: "Restart", DismissAfterRun: true}},
	}
}

// checkAction looks for a newer version now; the status row shows the check as it runs.
func checkAction(title string) api.Action {
	return api.Action{Kind: api.ActionUpdate, Op: api.UpdateOpCheck, Title: title}
}

// failureTitle names the step that failed: the check, or the install of the version it found.
func failureTitle(state api.UpdateState) string {
	if state.FailedStage == api.UpdateStageInstall {
		return "Could not install the update"
	}
	return "Could not check for updates"
}

// checkedSubtitle says when the last check finished and on which channel, or the schedule before the first check.
func checkedSubtitle(state api.UpdateState, now time.Time) string {
	channel := channelLabel(state.Channel) + " channel"
	if state.CheckedAtMs == 0 {
		return channel + " · " + checksEvery
	}
	return "Checked " + timeAgo(now, time.UnixMilli(state.CheckedAtMs)) + " · " + channel
}

// channelRow opens the list of channels. Its accessory names the channel the updater follows.
func channelRow(state api.UpdateState) api.Item {
	row := api.Item{
		ID: rowChannel, Title: "Update Channel", Icon: "shield-check",
		Accessories: []api.Accessory{{Text: channelLabel(state.Channel)}},
		Actions:     []api.Action{api.PushView("Change", channelsView(state))},
	}
	if state.Status == api.UpdateStatusUnsupported {
		row.Actions, row.Disabled = nil, true
	}
	return row
}
