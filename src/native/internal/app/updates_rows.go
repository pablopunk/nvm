package app

import (
	"strings"

	updatesbuiltin "github.com/pablopunk/nvm/native/internal/ext/builtins/updates"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/update"
)

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

// chooseUpdateChannel switches to the channel named, or to the next one (Stable, Nightly, Dev) when none is named, and
// checks the new channel at once. A switch is refused while a downloaded update waits for its restart.
func (p *App) chooseUpdateChannel(name string) error {
	if p.updates == nil {
		return update.ErrChannelUnavailable
	}
	channel := p.updates.Channel().Next()
	if name != "" {
		channel = update.ParseChannel(name)
	}
	if err := p.updates.SetChannel(channel); err != nil {
		return err
	}
	p.checkForUpdates(func(string) {})
	return nil
}

// compatibilityRootRows shows the compatibility row at the root while the backend refuses this client, in place of the
// Updates built-in's update row (see compatibilityReplacesUpdateRows). The rows stay out of the way of queries that do not name updates.
func compatibilityRootRows(p *App, _ *rootInput, q string) []search.Item {
	if !queryNamesUpdates(q) || !p.compat.rejectsClient() {
		return nil
	}
	return []search.Item{p.compatibilityRootItem(p.updateSnapshot().State)}
}

// compatibilityReplacesUpdateRows is true for the Updates built-in's rows while the compatibility row stands in their place.
func compatibilityReplacesUpdateRows(p *App, extensionID string) bool {
	return extensionID == updatesbuiltin.ID && p.compat.rejectsClient()
}

// queryNamesUpdates reports whether an empty query, or one that starts the word update, keeps the update rows on the root.
func queryNamesUpdates(q string) bool {
	lq := strings.ToLower(strings.TrimSpace(q))
	return lq == "" || strings.HasPrefix("update", lq) || strings.Contains("nevermind update", lq)
}

// rootShowsUpdate is whether the Updates built-in lists an update row at the root, which replaces Check for Updates.
func rootShowsUpdate(state update.State) bool {
	switch state.Status {
	case update.StatusAvailable, update.StatusDownloading, update.StatusDownloaded:
		return true
	case update.StatusFailed:
		return state.Stage == update.StageInstall
	}
	return false
}
