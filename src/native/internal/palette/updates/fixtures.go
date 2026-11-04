package updates

import "github.com/pablopunk/nvm/native/internal/update"

// States are the snapshots the goldens render: every status the page shows, on both channels where it matters.
func States() []Snapshot {
	return []Snapshot{
		{State: update.State{Status: update.StatusIdle, CurrentVersion: "2.0.0"}, Channel: update.ChannelStable},
		{State: update.State{Status: update.StatusCurrent, CurrentVersion: "2.0.0"}, Channel: update.ChannelStable},
		{State: update.State{Status: update.StatusChecking, CurrentVersion: "2.0.0"}, Channel: update.ChannelStable},
		{State: update.State{Status: update.StatusDownloading, CurrentVersion: "2.0.0", LatestVersion: "2.0.1", Progress: 0.42}, Channel: update.ChannelStable},
		{State: update.State{Status: update.StatusDownloaded, CurrentVersion: "2.0.0", LatestVersion: "2.0.1"}, Channel: update.ChannelNightly},
		{State: update.State{Status: update.StatusFailed, CurrentVersion: "2.0.0", Stage: update.StageInstall, Message: "the update is not signed with the app's key"}, Channel: update.ChannelStable},
		{State: update.State{Status: update.StatusUnsupported, CurrentVersion: "2.0.0", Message: "Updates are not available in this build yet"}, Channel: update.ChannelStable},
	}
}
