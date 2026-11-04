package app

import (
	"context"
	"errors"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/menubar"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/update"
)

// syncCompatibilityAccount follows the account, as 1.x does: the signed-in backend becomes the active one, and its
// manifest is fetched whenever the account changes. It runs on the UI thread.
func (p *App) syncCompatibilityAccount() {
	if p.account == nil {
		return
	}
	snap := p.account.Snapshot()
	active := ""
	if snap.Connected {
		active = snap.BaseURL
	}
	p.compat.setActive(active)
	p.refreshMenubarMenu()
	if active == "" || !p.compat.beginFetch() {
		return
	}
	client := p.account.Client()
	safego.Go("compatibility.fetch", func(ctx context.Context) {
		changed := p.fetchCompatibility(ctx, client, active)
		p.compat.endFetch()
		if changed {
			p.onUI(p.refreshMenubarMenu)
		}
	})
}

// fetchCompatibility asks the backend for its manifest and stores whether it accepts this client. A backend without a
// manifest, or one that cannot be reached, keeps the answer it had, as 1.x keeps its cache. It reports whether the
// answer changed.
func (p *App) fetchCompatibility(ctx context.Context, client *backend.Client, baseURL string) bool {
	if client == nil {
		return false
	}
	report, err := client.Compatibility(ctx)
	if err != nil || report.State == backend.CompatUnknown {
		return false
	}
	accepted := report.Manifest.Client.Compatible
	p.compat.persist(baseURL, accepted)
	return p.compat.record(baseURL, accepted)
}

// compatibilityUpdateRow is the update row while the backend refuses this client: "Update Nevermind" takes the place of
// the update row, and it is disabled where the updater cannot install (1.x would open the manifest's update URL, which
// points at the 1.x download, so 2.0 leaves it out until the 2.0 client name ships).
func (p *App) compatibilityUpdateRow(state update.State) *menubar.UpdateRow {
	if !p.compat.rejectsClient() {
		return nil
	}
	action := p.compatibilityAction(state)
	return &menubar.UpdateRow{Label: "Update Nevermind", Action: action, Disabled: action == nil}
}

// compatibilityAction is the click of the compatibility row for an updater state, or nil when it has none.
func (p *App) compatibilityAction(state update.State) func() {
	switch state.Status {
	case update.StatusDownloaded, update.StatusInstalling:
		return p.installFromMenu
	case update.StatusUnsupported:
		return nil
	}
	return p.openUpdatesAndCheck
}

// errClientRefused is what an AI or sign-in request says when the backend refuses this client. It names the update, as
// 1.x's refusal does.
var errClientRefused = errors.New("This Nevermind version is no longer supported by the backend. Update Nevermind to continue.")

// checkCompatibility fetches the manifest of the backend the account targets, then refuses the request when that
// manifest refuses this client. A fetch that fails leaves the answer it had, as 1.x does, so an offline check still
// judges by the last manifest. It runs off the UI thread.
func (p *App) checkCompatibility(ctx context.Context) error {
	if p.account == nil {
		return nil
	}
	snap := p.account.Snapshot()
	if client := p.account.Client(); client != nil && p.fetchCompatibility(ctx, client, snap.BaseURL) {
		p.onUI(p.refreshMenubarMenu)
	}
	return p.refusalFor(snap.BaseURL)
}

// refusalFor is errClientRefused when the known manifest of baseURL refuses this client, and nil otherwise.
func (p *App) refusalFor(baseURL string) error {
	if p.compat.refuses(baseURL) {
		return errClientRefused
	}
	return nil
}

// compatibilityRootItem is the palette root row while the backend refuses this client. It takes the place of the update
// row, and it runs the same click as the menu's compatibility row.
func (p *App) compatibilityRootItem(state update.State) search.Item {
	item := search.Item{
		Kind: search.KindCommands, Title: "Update Nevermind", Subtitle: "This version is no longer supported by the backend",
		Icon: "rotate-ccw", Score: compatibilityRootScore, LearnKey: "updates", Prominent: true,
	}
	if action := p.compatibilityAction(state); action != nil {
		item.Actions = []search.Action{{Title: "Update Nevermind", Run: func(search.Host) { action() }}}
	}
	return item
}

// compatibilityRootScore ranks the compatibility row above every other root row, as 1.x does.
const compatibilityRootScore = 1100
