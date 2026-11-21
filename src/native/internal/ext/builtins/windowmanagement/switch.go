package windowmanagement

import "github.com/pablopunk/nvm/native/internal/ext/api"

// switchLayouts are the layouts a window in the Switch Window list can take from its action menu.
var switchLayouts = []string{"left-half", "right-half", "maximize", "center", "restore"}

// switchView lists the windows of the running apps. Enter raises the chosen window; its action menu places it.
func switchView(listed []api.WindowInfo) api.View {
	items := make([]api.Item, 0, len(listed))
	for _, window := range listed {
		items = append(items, switchItem(window))
	}
	return api.ListView("Switch Window", items...)
}

func switchItem(window api.WindowInfo) api.Item {
	handle := string(window.Handle)
	title := window.Title
	if title == "" {
		title = window.AppName
	}
	focus := api.Run(focusWindowAction, "Focus", map[string]string{"handle": handle})
	actions := make([]api.Action, 0, len(switchLayouts))
	for _, id := range switchLayouts {
		if l, ok := layoutByID(id); ok {
			actions = append(actions, api.Run(applyWindowAction, l.presentation.title, map[string]string{"handle": handle, "layout": id}))
		}
	}
	return api.Item{ID: handle, Title: title, Subtitle: window.AppName, Icon: "app-window", Primary: &focus, Actions: actions}
}
