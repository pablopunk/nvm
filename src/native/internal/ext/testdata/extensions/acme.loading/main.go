package main

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"time"
)

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "list", Title: "Slow list fixture"}, {ID: "grid", Title: "Slow grid fixture"}, {ID: "compact", Title: "Slow compact fixture"}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID == "open" {
		return api.Result{View: &api.View{Kind: api.ViewList, Title: "Opened fixture", Items: []api.Item{{ID: "opened", Title: "Opened fixture"}}}}, nil
	}
	view := api.View{ID: in.Ref.ID, Kind: api.ViewList, Title: "Loading fixtures", Placeholder: "Filter fixtures", Source: &api.DataSource{Loader: in.Ref.ID, CacheKey: in.Ref.ID, TTLMs: 1, StaleTTLMs: 300000}}
	if in.Ref.ID != "list" {
		view.Kind = api.ViewGrid
		view.Grid = &api.GridSpec{Columns: 4, Layout: in.Ref.ID, SelectionFooter: true}
	}
	return api.Result{View: &view}, nil
}

func LoadItems(ctx api.Context, loader string) ([]api.Item, error) {
	time.Sleep(3 * time.Second)
	items := []api.Item{}
	for _, name := range []string{"Nevermind", "An unusually long repository name that must be clipped", "MyGo", "Palette", "Fixtures", "Design", "Window", "Search"} {
		glyph := "◆"
		if loader == "compact" {
			glyph = "😀"
		}
		items = append(items, api.Item{ID: name, Title: name, Subtitle: "pablopunk / Go", Icon: "folder", Glyph: glyph, Actions: []api.Action{{Title: "Open", Kind: api.ActionRun, Ref: &api.ActionRef{ID: "open"}}}})
	}
	return items, nil
}
