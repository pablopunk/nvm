package windows

import (
	"context"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/windowmgmt"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/commands"
)

// switchActionIDs are the layouts the window list offers on a row's action menu.
var switchActionIDs = []string{"window.left-half", "window.right-half", "window.maximize", "window.center", "window.restore"}

// switchView lists the windows of the running apps. Enter raises the chosen window; the action menu places it.
// Rows come from the providers, so the listing runs off the UI thread and refreshes in place.
func switchView(d Deps) *palette.View {
	return &palette.View{
		Title: "Switch Window", Placeholder: "Search windows",
		EmptyTitle: "No windows", EmptySub: "Open a window, then reopen Nevermind.",
		Providers: func() []search.Provider {
			return []search.Provider{search.NewRowsProvider("windows", func(ctx context.Context, q string) []search.Item {
				return windowRows(ctx, d, q)
			})}
		},
	}
}

// windowRows lists the windows whose title or app name contains q. A listing that fails says why in a row.
func windowRows(ctx context.Context, d Deps, q string) []search.Item {
	windows, err := d.Engine.ListWindows(ctx, windowmgmt.WindowQuery{})
	if err != nil {
		return []search.Item{{Kind: search.KindWindows, Title: "Could not list windows", Subtitle: err.Error(), Icon: "triangle-alert"}}
	}
	matched := []windowmgmt.WindowDescriptor{}
	for _, window := range windows {
		if matchesWindow(window, q) {
			matched = append(matched, window)
		}
	}
	subtitles := windowSubtitles(matched)
	rows := make([]search.Item, 0, len(matched))
	for i, window := range matched {
		rows = append(rows, windowRow(d, window, subtitles[i]))
	}
	return rows
}

// windowSubtitles names each row's app. A window whose app and title match another listed window is numbered among
// those, so two Terminal "~" windows read as Window 1 and Window 2.
func windowSubtitles(windows []windowmgmt.WindowDescriptor) []string {
	type identity struct{ app, title string }
	counts := map[identity]int{}
	for _, window := range windows {
		counts[identity{window.AppName, window.Title}]++
	}
	numbered := map[identity]int{}
	subtitles := make([]string, len(windows))
	for i, window := range windows {
		key := identity{window.AppName, window.Title}
		subtitles[i] = window.AppName
		if counts[key] > 1 {
			numbered[key]++
			subtitles[i] = fmt.Sprintf("%s · Window %d", window.AppName, numbered[key])
		}
	}
	return subtitles
}

func matchesWindow(window windowmgmt.WindowDescriptor, q string) bool {
	needle := strings.ToLower(q)
	return needle == "" || strings.Contains(strings.ToLower(window.Title), needle) ||
		strings.Contains(strings.ToLower(window.AppName), needle)
}

func windowRow(d Deps, window windowmgmt.WindowDescriptor, subtitle string) search.Item {
	title := window.Title
	if title == "" {
		title = window.AppName
	}
	actions := []search.Action{{Title: "Focus Window", Dismiss: search.DismissAlways, Run: focusAction(d, window.Handle)}}
	for _, id := range switchActionIDs {
		actions = append(actions, layoutAction(d, id, window.Handle))
	}
	return search.Item{
		Kind: search.KindWindows, Title: title, Subtitle: subtitle, Icon: "app-window",
		Accessory: stateOf(window), Actions: actions,
	}
}

func stateOf(window windowmgmt.WindowDescriptor) string {
	switch {
	case window.FullScreen:
		return "Full screen"
	case window.Minimized:
		return "Minimized"
	}
	return ""
}

func focusAction(d Deps, handle string) func(search.Host) {
	return func(search.Host) {
		d.runInBackground("focus", "Focus Window", func(ctx context.Context) string {
			return messageOf(d.Engine.FocusWindow(ctx, handle))
		})
	}
}

func layoutAction(d Deps, id, handle string) search.Action {
	command, _ := commands.ByID(id)
	return search.Action{Title: command.Title, Dismiss: search.DismissAlways, Run: func(search.Host) {
		d.runInBackground("window-layout", command.Title, func(ctx context.Context) string {
			return d.runner().ExecuteOnWindow(ctx, command, handle)
		})
	}}
}

func messageOf(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
