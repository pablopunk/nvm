// Package syspanes is the built-in that finds the operating system's settings panes from the root and opens them.
// It is the first built-in moved onto the extension API; its rows and actions match the native rows it replaces.
//
// Layer: ext. The package imports api, search and the standard library.
package syspanes

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Extension is the System Settings built-in.
type Extension struct{}

// settingsPaneSubtitle labels the rows, as the OS names its settings app.
const settingsPaneSubtitle = "System Settings"

// settingsPaneIcon is the tile of every pane row.
const settingsPaneIcon = "settings"

// paneAliases are the other names a pane answers to from the root, as the Electron app lists them: "iCloud" finds
// Apple Account and "Wireless" finds Wi-Fi.
var paneAliases = map[string][]string{
	"com.apple.systempreferences.AppleIDSettings":    {"Apple ID", "iCloud"},
	"com.apple.wifi-settings-extension":              {"WiFi", "Wireless"},
	"com.apple.Siri-Settings.extension":              {"Siri"},
	"com.apple.settings.PrivacySecurity.extension":   {"Privacy", "Security"},
	"com.apple.Desktop-Settings.extension":           {"Desktop", "Dock"},
	"com.apple.Touch-ID-Settings.extension":          {"Touch ID", "Password"},
	"com.apple.Users-Groups-Settings.extension":      {"Users", "Groups"},
	"com.apple.Internet-Accounts-Settings.extension": {"Accounts"},
	"com.apple.CD-DVD-Settings.extension":            {"CD", "DVD"},
	"com.apple.Print-Scan-Settings.extension":        {"Printers", "Scanners"},
}

// Manifest identifies the built-in. Its rows are listed under the Commands section, as the native rows were.
func (Extension) Manifest() api.Manifest {
	return api.Manifest{APIVersion: "1.0", Identity: api.Identity{ID: "nevermind.system-settings", Title: "System Settings"}}
}

// SearchItems lists the panes whose titles or aliases match the query. Panes show only for a query, never at the
// empty root.
func (Extension) SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	lq := strings.ToLower(query)
	if lq == "" {
		return nil, nil
	}
	var out []api.Item
	for _, pane := range ctx.System().SettingsPanes() {
		score := paneSearchScore(pane, lq)
		if score == 0 {
			continue
		}
		out = append(out, paneItem(pane, score))
	}
	return out, nil
}

// paneSearchScore is the best score of the query against the pane's title and its aliases; 0 means no match.
func paneSearchScore(pane api.SettingsPane, lq string) int {
	best := search.Score(pane.Title, lq)
	for _, alias := range paneAliases[pane.ID] {
		best = max(best, search.Score(alias, lq))
	}
	return best
}

// paneItem is the row of one pane: its title, the System Settings subtitle, and an Open action that opens it.
func paneItem(pane api.SettingsPane, score int) api.Item {
	return api.Item{
		ID: pane.ID, Title: pane.Title, Subtitle: settingsPaneSubtitle, Icon: settingsPaneIcon,
		Score: float64(score), Section: search.KindCommands,
		Primary: &api.Action{Kind: api.ActionSystem, Op: "open-system-settings", Setting: pane.ID, Title: "Open"},
	}
}
