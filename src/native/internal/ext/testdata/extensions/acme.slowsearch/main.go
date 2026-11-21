package main

import (
	"strings"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.slowsearch is the dev fixture of a grid and a list whose onSearchChange answers late. The host shows the second
// batch as loading while the handler runs, with a placeholder after the rows, and the answer replaces it in place.

// slowSearchDelay is how long the handler takes to answer, as a model-backed search does.
const slowSearchDelay = time.Second

var fruit = []api.Item{
	{ID: "apple", Title: "Apple", Subtitle: "Red", Glyph: "🍎"},
	{ID: "banana", Title: "Banana", Subtitle: "Yellow", Glyph: "🍌"},
	{ID: "cherry", Title: "Cherry", Subtitle: "Red", Glyph: "🍒"},
	{ID: "lemon", Title: "Lemon", Subtitle: "Yellow", Glyph: "🍋"},
}

var slowGrid = api.View{
	Kind: api.ViewGrid, Title: "Slow search grid", Placeholder: "Search fruit", Grid: &api.GridSpec{Columns: 4},
	Items:          fruit,
	OnSearchChange: &api.ActionRef{ID: "slow-search"},
	EmptyView:      &api.EmptyState{Title: "No fruit found", Subtitle: "Try yellow."},
}

var slowList = api.View{
	Kind: api.ViewList, Title: "Slow search list", Placeholder: "Search fruit",
	Items:          fruit,
	OnSearchChange: &api.ActionRef{ID: "slow-search"},
	EmptyView:      &api.EmptyState{Title: "No fruit found", Subtitle: "Try yellow."},
}

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{
		{ID: "grid", Title: "Slow search grid fixture"},
		{ID: "list", Title: "Slow search list fixture"},
	}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "grid":
		return api.Result{View: &slowGrid}, nil
	case "list":
		return api.Result{View: &slowList}, nil
	case "slow-search":
		time.Sleep(slowSearchDelay)
		return api.Result{Patch: &api.ViewPatch{SearchResultIds: slowMatches(in.Value)}}, nil
	}
	return api.Result{}, nil
}

// slowMatches answers the second batch: a search for "yellow" finds the fruit whose colour is yellow.
func slowMatches(text string) []string {
	if strings.Contains(strings.ToLower(text), "yellow") {
		return []string{"banana", "lemon"}
	}
	return nil
}
