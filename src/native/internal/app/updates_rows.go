package app

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/updates"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/update"
)

// updateSnapshot is what the Updates page and the Settings rows show. Without an updater it is unsupported on stable.
func (p *App) updateSnapshot() updates.Snapshot {
	if p.updates == nil {
		return updates.Snapshot{State: update.NoUpdates{}.State(), Channel: update.ChannelStable}
	}
	return updates.Snapshot{State: p.updates.State(), Channel: p.updates.Channel()}
}

// updateAccessory is the short status the Settings row shows at its right edge.
func updateAccessory(state update.State) string {
	switch state.Status {
	case update.StatusChecking:
		return "Checking"
	case update.StatusAvailable, update.StatusDownloading:
		return "Downloading " + state.LatestVersion
	case update.StatusDownloaded, update.StatusInstalling:
		return "Restart to update"
	case update.StatusFailed:
		return "Failed"
	case update.StatusUnsupported:
		return "Unavailable"
	case update.StatusCurrent:
		return "Up to date"
	}
	return "Not checked"
}

// toggleUpdateChannel switches between stable and nightly and checks the new channel at once. A switch is refused
// while a downloaded update waits for its restart, and in a build that cannot update, with the reason.
func (p *App) toggleUpdateChannel() error {
	if p.updates == nil {
		return update.ErrChannelUnavailable
	}
	if err := p.updates.SetChannel(p.updates.Channel().Other()); err != nil {
		return err
	}
	p.checkForUpdates(func(string) {})
	return nil
}

// updatesView is the Updates page, pushed from Settings and the root.
func (p *App) updatesView() *palette.View {
	return updates.View(p.updateSnapshot, updates.Actions{
		Check:         func() { p.checkForUpdates(func(string) {}) },
		Restart:       p.restartToUpdate,
		ToggleChannel: p.toggleUpdateChannel,
	})
}

// updateRootRows shows an update that is found, downloading or ready at the root, above the commands, where the person
// is about to search. A failed install is shown too, so a refused update is never silent; a failed check is not, because
// an offline laptop would show it at every launch. The rows stay out of the way of queries that do not name updates.
func updateRootRows(p *App, _ *rootInput, q string) []search.Item {
	state := p.updateSnapshot().State
	if !queryNamesUpdates(q) {
		return nil
	}
	if p.compat.rejectsClient() {
		return []search.Item{p.compatibilityRootItem(state)}
	}
	if !rootShowsUpdate(state) {
		return nil
	}
	item := search.Item{Kind: search.KindCommands, Title: update.Message(state), Subtitle: "Nevermind updates", Icon: "rotate-ccw",
		Score: rootUpdateScore, LearnKey: "updates", Prominent: state.Status == update.StatusDownloaded,
		Actions: []search.Action{{Title: "Open Updates", Run: func(h search.Host) { h.Push(p.updatesView()) }}}}
	if state.Status == update.StatusDownloaded {
		item.Actions = []search.Action{{Title: "Restart to Update", Run: func(h search.Host) {
			if err := p.restartToUpdate(); err != nil {
				h.Toast(err.Error())
			}
		}}}
	}
	return []search.Item{item}
}

// queryNamesUpdates reports whether an empty query, or one that starts the word update, keeps the update rows on the root.
func queryNamesUpdates(q string) bool {
	lq := strings.ToLower(strings.TrimSpace(q))
	return lq == "" || strings.HasPrefix("update", lq) || strings.Contains("nevermind update", lq)
}

// rootUpdateScore ranks an update row above every command, so it is the first row of the root.
const rootUpdateScore = 95

func rootShowsUpdate(state update.State) bool {
	switch state.Status {
	case update.StatusAvailable, update.StatusDownloading, update.StatusDownloaded:
		return true
	case update.StatusFailed:
		return state.Stage == update.StageInstall
	}
	return false
}
