package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/update"
)

// fixedUpdates is the updates surface with one state, so a golden shows the state its fixture names.
type fixedUpdates struct {
	state api.UpdateState
}

func (f fixedUpdates) State() api.UpdateState { return f.state }

// updatesGoldenStates are the updater states the goldens render: every status the page shows, on the channel each one shows.
var updatesGoldenStates = []updateStatus{
	{State: update.State{Status: update.StatusIdle, CurrentVersion: "2.0.0"}, Channel: update.ChannelStable},
	{State: update.State{Status: update.StatusCurrent, CurrentVersion: "2.0.0"}, Channel: update.ChannelStable},
	{State: update.State{Status: update.StatusChecking, CurrentVersion: "2.0.0"}, Channel: update.ChannelStable},
	{State: update.State{Status: update.StatusDownloading, CurrentVersion: "2.0.0", LatestVersion: "2.0.1", Progress: 0.42}, Channel: update.ChannelStable},
	{State: update.State{Status: update.StatusDownloaded, CurrentVersion: "2.0.0", LatestVersion: "2.0.1"}, Channel: update.ChannelNightly},
	{State: update.State{Status: update.StatusFailed, CurrentVersion: "2.0.0", Stage: update.StageInstall, Message: "the update is not signed with the app's key"}, Channel: update.ChannelStable},
	{State: update.State{Status: update.StatusUnsupported, CurrentVersion: "2.0.0", Message: "Updates are not available in this build yet"}, Channel: update.ChannelStable},
}

// TestUpdatesPageGoldens renders the Updates built-in's page in each status the fixtures list, on the channel each one shows.
func TestUpdatesPageGoldens(t *testing.T) {
	for _, status := range updatesGoldenStates {
		p := goldenPalette(t)
		p.exts.mgr.Store(newFixtureManagerWithUpdates(t, p, fixedUpdates{state: apiUpdateState(status)}))
		goldenOpen(p, p.updatesPage())
		golden.AssertMatrix(t, "updates-"+string(status.State.Status)+"-"+string(status.Channel), p.View, paletteGoldens)
	}
}
