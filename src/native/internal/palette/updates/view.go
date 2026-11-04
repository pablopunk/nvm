// Package updates is the Updates page: what the updater knows, the actions a person takes on it (check now, restart
// into a downloaded update, switch the channel), and the one-line status the Settings row and the root show. The page
// reads its state live, so a download in progress shows its progress on the next query.
//
// Layer: palette. It imports palette, search and update; the app wires the actions.
package updates

import (
	"slices"
	"strings"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/update"
)

// Snapshot is what the page shows at one moment.
type Snapshot struct {
	State   update.State
	Channel update.Channel
}

// Actions are what the rows do. Restart and ToggleChannel return the reason they refuse, shown as a toast.
type Actions struct {
	Check         func()
	Restart       func() error
	ToggleChannel func() error
}

// checkEvery is the cadence the page states; it is update.CheckInterval in words.
const checkEvery = "Checks every 4 hours"

// View is the Updates page. read is called on every query, on the UI thread.
func View(read func() Snapshot, a Actions) *palette.View {
	return &palette.View{
		Title:       "Updates",
		Placeholder: "Search updates",
		Hint:        "Updates install after a restart",
		Search: func(q string) []search.Item {
			return filter(rows(read(), a), q)
		},
	}
}

// StatusIcon is the icon of the status row.
func StatusIcon(status update.Status) string {
	switch status {
	case update.StatusFailed:
		return "circle-alert"
	case update.StatusAvailable, update.StatusDownloading:
		return "arrow-down"
	case update.StatusDownloaded, update.StatusInstalling:
		return "rotate-ccw"
	}
	return "check"
}

// StatusLine is the one line that says where the updater stands; the Settings row and the root show it too.
func StatusLine(s update.State) string {
	return update.Message(s)
}

func rows(in Snapshot, a Actions) []search.Item {
	items := []search.Item{{
		Kind:       search.KindCommands,
		Title:      StatusLine(in.State),
		Subtitle:   statusSubtitle(in),
		Icon:       StatusIcon(in.State.Status),
		Contextual: in.State.Status == update.StatusFailed,
		Disabled:   true,
	}}
	if in.State.Status == update.StatusDownloaded {
		items = append(items, search.Item{Kind: search.KindCommands, Title: "Restart to Update", Subtitle: "Nevermind " + in.State.LatestVersion + " is installed; restart to run it", Icon: "rotate-ccw", Prominent: true,
			Actions: []search.Action{{Title: "Restart", Run: func(h search.Host) {
				if err := a.Restart(); err != nil {
					h.Toast(err.Error())
				}
			}}}})
	}
	items = append(items,
		search.Item{Kind: search.KindCommands, Title: "Check for Updates", Subtitle: "Look for a newer version now", Icon: "rotate-ccw",
			Actions: []search.Action{{Title: "Check", Dismiss: search.DismissNever, Run: func(h search.Host) {
				a.Check()
				h.Refresh()
			}}}},
		search.Item{Kind: search.KindCommands, Title: "Update Channel", Subtitle: channelSubtitle(in), Accessory: in.Channel.Label(), Icon: "shield-check",
			Actions: []search.Action{{Title: "Switch", Dismiss: search.DismissNever, Run: func(h search.Host) {
				if err := a.ToggleChannel(); err != nil {
					h.Toast(err.Error())
				}
				h.Refresh()
			}}}},
	)
	return items
}

// statusSubtitle names the channel and the schedule, but only in a build that checks: an unsupported build checks nothing.
func statusSubtitle(in Snapshot) string {
	if in.State.Status == update.StatusUnsupported {
		return ""
	}
	return in.Channel.Label() + " channel · " + checkEvery
}

func channelSubtitle(in Snapshot) string {
	switch {
	case in.State.Status == update.StatusUnsupported:
		return "Not available in this build yet"
	case in.Channel == update.ChannelNightly:
		return "Includes nightly prereleases; a final release still wins when newer"
	}
	return "Final releases only"
}

func filter(items []search.Item, q string) []search.Item {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return items
	}
	return slices.DeleteFunc(items, func(item search.Item) bool {
		return !strings.Contains(strings.ToLower(item.Title+" "+item.Subtitle), q)
	})
}
