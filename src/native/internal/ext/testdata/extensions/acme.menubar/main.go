package main

import (
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.menubar is the dev fixture of menu bar items. Two items share one extension: "main" ticks once a second and
// carries sections, a submenu and the actions that change its own state; "status" re-renders only when asked. The
// "break" action makes the render fail, which shows the not-responding head and keeps the last good rows, and "fix"
// clears it. Links and copy rows exercise the host operations a menu row may run.

func MenuBarTriggers() []api.Trigger {
	return []api.Trigger{{Type: api.TriggerInterval, Every: "1s"}, {Type: api.TriggerStartup}}
}

func MenuBars(ctx api.Context) ([]api.MenuBar, error) {
	broken, err := isBroken(ctx)
	if err != nil {
		return nil, err
	}
	if broken {
		return nil, errors.New("the fixture was told to fail")
	}
	count, err := readCount(ctx)
	if err != nil {
		return nil, err
	}
	return []api.MenuBar{mainBar(ctx, count), statusBar(count)}, nil
}

func mainBar(ctx api.Context, count int) api.MenuBar {
	tick := ctx.Clock().Now().Second()
	bump := api.Run("bump", "Bump count", nil)
	brk := api.Run("break", "Break render", nil)
	fix := api.Run("fix", "Fix render", nil)
	openDocs := api.OpenURL("Open documentation", "https://example.com/docs")
	copyCount := api.CopyText("Copy count", fmt.Sprintf("%d", count), false)
	return api.MenuBar{
		ID:      "main",
		Icon:    "timer",
		Title:   fmt.Sprintf("tick %02d", tick),
		Tooltip: "Menu bar fixture",
		Sections: []api.MenuSection{
			{Title: "Counter", Rows: []api.MenuRow{
				{ID: "count", Title: fmt.Sprintf("Count %d", count), Disabled: true},
				{ID: "bump", Title: "Bump", Action: &bump},
			}},
			{Title: "Failure", Rows: []api.MenuRow{
				{ID: "break", Title: "Break", Action: &brk},
				{ID: "fix", Title: "Fix", Action: &fix},
			}},
			{Rows: []api.MenuRow{
				{ID: "more", Title: "More", Items: []api.MenuRow{
					{ID: "copy", Title: "Copy count", Action: &copyCount},
					{ID: "docs", Title: "Documentation", Action: &openDocs},
				}},
			}},
		},
	}
}

func statusBar(count int) api.MenuBar {
	return api.MenuBar{
		ID:      "status",
		Icon:    "circle-dot",
		Title:   fmt.Sprintf("%d", count),
		Tooltip: "Count",
		Sections: []api.MenuSection{{Rows: []api.MenuRow{
			{ID: "count", Title: fmt.Sprintf("Counted %d", count), Disabled: true},
		}}},
	}
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "bump":
		count, err := readCount(ctx)
		if err != nil {
			return api.Result{}, err
		}
		if err := ctx.Storage().Set("count", count+1); err != nil {
			return api.Result{}, err
		}
	case "break":
		if err := ctx.Storage().Set("broken", true); err != nil {
			return api.Result{}, err
		}
	case "fix":
		if err := ctx.Storage().Set("broken", false); err != nil {
			return api.Result{}, err
		}
	default:
		return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
	}
	ctx.Views().RefreshMenuBar()
	return api.Result{}, nil
}

func readCount(ctx api.Context) (int, error) {
	var count int
	_, err := ctx.Storage().Get("count", &count)
	return count, err
}

func isBroken(ctx api.Context) (bool, error) {
	var broken bool
	_, err := ctx.Storage().Get("broken", &broken)
	return broken, err
}
