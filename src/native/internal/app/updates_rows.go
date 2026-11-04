package app

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/menubar"
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
	}
	return "Up to date"
}

// toggleUpdateChannel switches to the next channel (Stable, Nightly, Dev) and checks the new channel at once. A switch is refused
// while a downloaded update waits for its restart.
func (p *App) toggleUpdateChannel() error {
	if p.updates == nil {
		return nil
	}
	if err := p.updates.SetChannel(p.updates.Channel().Next()); err != nil {
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

// menubarUpdateRow is the menu's update row while an update is found, downloading or ready.
func (p *App) menubarUpdateRow() *menubar.UpdateRow {
	state := p.updateSnapshot().State
	switch state.Status {
	case update.StatusAvailable, update.StatusDownloading:
		return &menubar.UpdateRow{Label: update.Message(state)}
	case update.StatusDownloaded:
		return &menubar.UpdateRow{Label: "Restart to Update to " + state.LatestVersion, Action: func() { _ = p.restartToUpdate() }}
	}
	return nil
}

// updateRootRows shows an update that is found, downloading or ready at the root, above the commands, where the person
// is about to search. A failed install is shown too, so a refused update is never silent; a failed check is not, because
// an offline laptop would show it at every launch. The rows stay out of the way of queries that do not name updates.
func updateRootRows(p *App, _ *rootInput, q string) []search.Item {
	state := p.updateSnapshot().State
	if !rootShowsUpdate(state) {
		return nil
	}
	if lq := strings.ToLower(strings.TrimSpace(q)); lq != "" && !strings.HasPrefix("update", lq) && !strings.Contains("nevermind update", lq) {
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

// rootUpdateScore is the update prompt's score, as 1.x's update prompt has it: above every other root row.
const rootUpdateScore = 1000

func rootShowsUpdate(state update.State) bool {
	switch state.Status {
	case update.StatusAvailable, update.StatusDownloading, update.StatusDownloaded:
		return true
	case update.StatusFailed:
		return state.Stage == update.StageInstall
	}
	return false
}
