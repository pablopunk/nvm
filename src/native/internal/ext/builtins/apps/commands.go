package apps

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// The two commands of the app list. Their IDs are the 1.x command IDs, so their shortcuts and aliases keep their keys.
const (
	ForceQuitCommandID = "force-quit-apps"
	QuitAllCommandID   = "quit-all-apps"
)

// Titles, subtitles and the root scores of the commands, as 1.x shows them. Force Quit Apps ranks above the other
// commands at the empty root, and Quit All Apps at the ordinary command score.
const (
	forceQuitTitle    = "Force Quit Apps"
	forceQuitSubtitle = "Force quit running applications"
	forceQuitKeywords = "force quit kill processes apps running"
	forceQuitScore    = 14
	quitAllTitle      = "Quit All Apps"
	quitAllSubtitle   = "Closes all visible GUI applications"
	quitAllKeywords   = "quit all apps close everything"
	quitAllScore      = 12
	forceQuitItemSub  = "Force quit this application"
	quitAllMessage    = "This will close all visible applications. Unsaved changes may be lost."
	quitAllStarted    = "Quitting applications..."
	noRunningApps     = "No running apps"
	noRunningSubtitle = "No running applications to force quit."
)

// Commands lists Force Quit Apps and Quit All Apps. Both are customizable, so they take a shortcut and an alias.
func (Extension) Commands(api.Context) ([]api.Command, error) {
	return []api.Command{
		{
			ID: ForceQuitCommandID, Title: forceQuitTitle, Subtitle: forceQuitSubtitle, Icon: "stop-circle",
			Keywords: splitWords(forceQuitKeywords), Score: forceQuitScore,
			Appearance: &api.Appearance{Foreground: api.ColorRed},
			Section:    search.KindCommands, LearnKey: "cmd:" + forceQuitTitle,
		},
		{
			ID: QuitAllCommandID, Title: quitAllTitle, Subtitle: quitAllSubtitle, Icon: "power",
			Keywords: splitWords(quitAllKeywords), Score: quitAllScore,
			Appearance: &api.Appearance{Foreground: api.ColorRed},
			Section:    search.KindCommands, LearnKey: "cmd:" + quitAllTitle,
		},
	}, nil
}

// forceQuitCommand opens the list of running apps, each with its force quit behind a confirmation.
func forceQuitCommand(ctx api.Context) (api.Result, error) {
	list, err := ctx.Desktop().Apps().List()
	if err != nil {
		return api.Result{}, err
	}
	running := runningAmong(ctx, list)
	var items []api.Item
	for _, app := range list {
		if running[app.Path] {
			items = append(items, forceQuitItem(app))
		}
	}
	view := api.ListView(forceQuitTitle, items...)
	view.Placeholder = "Search running apps"
	view.EmptyView = &api.EmptyState{Title: noRunningApps, Subtitle: noRunningSubtitle}
	return ctx.Navigation().Push(&view), nil
}

// forceQuitItem is the row of one running app in the force quit list. Its only action is the force quit, which asks.
func forceQuitItem(app api.App) api.Item {
	quit := api.Destructive(api.Run(refForceQuit, "Force Quit "+app.Name, map[string]string{argPath: app.Path, argName: app.Name}))
	quit = api.WithConfirmation(quit, api.Confirmation{
		Title: "Force Quit?", Message: "Force quit " + app.Name + "? Unsaved data may be lost.",
		ConfirmLabel: "Force Quit", Destructive: true,
	})
	return api.Item{
		Title: app.Name, Subtitle: forceQuitItemSub, File: &api.FileRef{Path: app.Path, Name: app.Name},
		Primary: &quit,
	}
}

// forceQuit ends one app at once, after the confirmation the row asked, and says what happened.
func forceQuit(ctx api.Context, path, name string) (api.Result, error) {
	outcome, err := ctx.Desktop().Apps().ForceQuit(path)
	if err != nil {
		return api.Result{}, err
	}
	return toast(quitMessage(name, true, outcome)), nil
}

// quitAllCommand asks once before every visible app is asked to quit, as 1.x does.
func quitAllCommand() (api.Result, error) {
	confirmed := api.Run(refQuitAll, quitAllTitle, nil)
	confirmed = api.WithConfirmation(confirmed, api.Confirmation{
		Title: quitAllTitle + "?", Message: quitAllMessage, ConfirmLabel: quitAllTitle, Destructive: true,
	})
	return api.Result{Next: &confirmed}, nil
}

// quitAll asks every running app but Nevermind and the desktop's own apps to quit.
func quitAll(ctx api.Context) (api.Result, error) {
	if _, err := ctx.Desktop().Apps().QuitAll(); err != nil {
		return api.Result{}, err
	}
	return toast(quitAllStarted), nil
}
