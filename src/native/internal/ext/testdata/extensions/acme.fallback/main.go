package main

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// rootRuns counts the RootItems calls, so a root refresh shows as a new row title.
var rootRuns int

// SearchItems offers one query fallback for every query, as a web search does. Its Score is set high on purpose: the
// host must ignore it for a fallback row and place the row in the fallback band.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	return []api.Item{{
		ID:            "fallback",
		Title:         `Search the fixture for "` + query + `"`,
		Subtitle:      "Query fallback",
		QueryFallback: true,
		Score:         990,
		Primary:       api.Primary(api.CopyText("Copy", query, false)),
	}}, nil
}

// RootItems returns the root rows. The fallback row must never appear here, and the run row shows how often the root
// asked for its rows.
func RootItems(ctx api.Context) ([]api.Item, error) {
	rootRuns++
	return []api.Item{
		{ID: "root", Title: "Fallback root", QueryFallback: true},
		{ID: "runs", Title: fmt.Sprintf("Root run %d", rootRuns)},
	}, nil
}

// RunAction handles bump: it asks the host to run the root's rows again.
func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID == "invalidate-view" {
		ctx.Views().Invalidate()
		return api.Result{}, nil
	}
	ctx.Views().InvalidateRoot()
	return api.Result{}, nil
}
