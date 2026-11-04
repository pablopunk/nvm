package app

import (
	"context"
	"strings"

	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// settingsPaneSubtitle labels the settings panes in the root, as the OS names its settings app.
const settingsPaneSubtitle = "System Settings"

// settingsPaneRows lists the System Settings panes that match a query. The panes show only for a query, never in
// the empty root, and only where the OS has them.
func settingsPaneRows(p *App, _ *rootInput, q string) []search.Item {
	lq := strings.ToLower(q)
	if lq == "" {
		return nil
	}
	var out []search.Item
	for _, pane := range p.systemSvc.SettingsPanes() {
		score := search.Score(pane.Title, lq)
		if score == 0 {
			continue
		}
		out = append(out, search.Item{
			Kind: search.KindCommands, Title: pane.Title, Subtitle: settingsPaneSubtitle, Icon: "settings", Score: score,
			Actions: []search.Action{{Title: "Open", Run: func(h search.Host) {
				h.Hide()
				openSettingsPane(p, pane)
			}}},
		})
	}
	return out
}

// openSettingsPane opens one settings pane off the UI thread.
func openSettingsPane(p *App, pane systemfeat.Pane) {
	safego.Go("system.pane", func(context.Context) {
		if err := p.systemSvc.OpenPane(pane, ""); err != nil {
			p.toastFromWorker(describeSystemFailure(pane.Title, err))
		}
	})
}
