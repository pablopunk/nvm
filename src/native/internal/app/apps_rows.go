package app

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/apps"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// applicationRows matches installed applications by name, and at the root shows the ones used before. The rows and
// their actions come from the apps feature; this function scores them and maps the actions onto the palette.
func applicationRows(p *App, in *rootInput, q string) []search.Item {
	if p.appsvc == nil {
		return nil
	}
	lq := strings.ToLower(q)
	var list []apps.App
	var scores []int
	for _, a := range in.apps {
		s := 0
		if lq == "" {
			if in.learn.Uses(appLearnKey(a)) == 0 {
				continue
			}
			s = scoreAppSuggestion
		} else if s = search.Score(a.Name, lq); s == 0 {
			continue
		}
		list = append(list, a)
		scores = append(scores, s)
	}
	rows := apps.AppRows(list, p.appsvc.Running.RunningAmong(list), p.appsvc.SupportsUninstall())
	out := make([]search.Item, 0, len(rows))
	for i, row := range rows {
		out = append(out, p.withShortcutActions(p.appItem(list[i], row, scores[i]), appShortcutID(list[i].ID), in.shortcuts))
	}
	return out
}

// appLearnKey names an app to the usage learner. It is the same key 1.x and the prototype used, so learning carries over.
func appLearnKey(a apps.App) string {
	return "app:" + a.ID
}

// appItem turns a feature row into a palette row. The row's first action is the one Enter runs.
func (p *App) appItem(a apps.App, row apps.Row, score int) search.Item {
	it := search.Item{
		Kind:     search.KindApplications,
		Title:    row.Title,
		Subtitle: row.Subtitle,
		IconPath: row.Path,
		Score:    score,
		LearnKey: appLearnKey(a),
		Actions:  make([]search.Action, 0, len(row.Actions)),
	}
	for _, act := range row.Actions {
		it.Actions = append(it.Actions, p.appAction(a, act))
	}
	return it
}

// appAction maps one feature action onto the palette. Destructive actions confirm first; long work runs off the UI
// thread and reports through a toast or a result view.
func (p *App) appAction(a apps.App, act apps.Action) search.Action {
	switch act.Kind {
	case apps.ActionOpen:
		return search.Action{Title: act.Title, Run: func(h search.Host) {
			hideOrReport(h, p.ports.Shell.LaunchApp(shellApp(a)))
		}}
	case apps.ActionReveal:
		return search.Action{Title: act.Title, Label: "⌃↵", Mods: mod, Key: ui.KeyEnter, Run: func(h search.Host) {
			h.Hide()
			_ = p.ports.Shell.Reveal(a.Path)
		}}
	case apps.ActionCopyPath:
		return search.Action{Title: act.Title, Run: func(h search.Host) {
			mygo.Clipboard.WriteText(a.Path)
			h.Hide()
		}}
	case apps.ActionQuit:
		return search.Action{Title: act.Title, Dismiss: search.DismissNever, Run: func(h search.Host) {
			p.quitApps([]apps.App{a}, false)
			h.Toast("Quitting " + a.Name + "…")
		}}
	case apps.ActionForceQuit:
		return search.Action{Title: act.Title, Run: func(h search.Host) {
			h.Push(palette.Confirm(act.Confirm, act.ConfirmMessage, func() { p.quitApps([]apps.App{a}, true) }))
		}}
	case apps.ActionUninstall:
		return search.Action{Title: act.Title, Run: func(h search.Host) {
			h.Push(p.uninstallView(a))
		}}
	}
	return search.Action{Title: act.Title, Run: func(search.Host) {}}
}

// shellApp is the platform's description of an app, for the Shell port.
func shellApp(a apps.App) platform.App {
	return platform.App{Name: a.Name, ID: a.ID, Path: a.Path}
}
