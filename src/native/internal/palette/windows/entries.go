package windows

import (
	"context"
	"strings"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/commands"
)

// Subtitle is the line every window command shows under its title, as in the Electron app.
const Subtitle = "Window Management"

// Entry is one command of the palette's command list. The app turns each into its own command row, so window
// commands sort, learn and dismiss like the other commands. ID is stable and is what shortcut bindings name.
type Entry struct {
	ID       string
	Title    string
	Subtitle string
	Icon     string
	Keywords string
	// Hidden entries answer only typed queries, as the other palette commands do.
	Hidden bool
	Run    func(h search.Host)
}

// Entries lists the window commands: the layouts and focus moves of the catalog, then the views.
func Entries(d Deps) []Entry {
	catalog := commands.All()
	entries := make([]Entry, 0, len(catalog)+viewCount)
	for _, command := range catalog {
		entries = append(entries, commandEntry(d, command))
	}
	return append(entries, viewEntries(d)...)
}

func commandEntry(d Deps, command commands.Command) Entry {
	return Entry{
		ID:       command.ID,
		Title:    command.Title,
		Subtitle: Subtitle,
		Icon:     command.Icon,
		Keywords: strings.Join(command.Keywords, " "),
		Hidden:   true,
		Run: func(search.Host) {
			invocation := d.paletteInvocation()
			d.runInBackground("command", command.Title, func(ctx context.Context) string {
				return d.runner().Execute(ctx, command, invocation)
			})
		},
	}
}

func viewEntries(d Deps) []Entry {
	return append(baseViewEntries(d), exactEntries(d)...)
}

func baseViewEntries(d Deps) []Entry {
	return []Entry{
		{
			ID: switchEntryID, Title: "Switch Window", Subtitle: Subtitle, Icon: "app-window", Hidden: true,
			Keywords: "window switch focus raise bring front list",
			Run:      func(h search.Host) { h.Push(switchView(d)) },
		},
		{
			ID: settingsEntryID, Title: "Window Management Settings", Subtitle: "Gaps, cycling, and sizes for window layouts",
			Icon: "settings", Hidden: true, Keywords: "window management settings gap size layout",
			Run: func(h search.Host) { h.Push(settingsView(d)) },
		},
		{
			ID: customEntryID, Title: "Custom Window Commands", Subtitle: Subtitle, Icon: "layout-list", Hidden: true,
			Keywords: "custom window commands raycast",
			Run:      func(h search.Host) { h.Push(customView()) },
		},
	}
}

const viewCount = 5

const (
	switchEntryID   = "window.switch"
	settingsEntryID = "window.settings"
	customEntryID   = "window.custom"
)

// customNotice is the Electron app's copy for Custom Window Commands: a pointer to AI, not a command editor.
const customNotice = "You don't need custom window commands here. Describe the layout in the palette, for example " +
	"“open Safari on the left at 60% and Terminal on the right at 40%”, and AI will build an extension for it."

func customView() *palette.View {
	return &palette.View{
		Title: "Custom Window Commands", Placeholder: "Custom Window Commands",
		Search: func(string) []search.Item {
			return []search.Item{{Kind: search.KindCommands, Title: "Ask AI to build it", Subtitle: customNotice, Icon: "sparkles"}}
		},
	}
}
