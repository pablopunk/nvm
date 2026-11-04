package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

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

// RootItems is the one root row. A query fallback must never appear here.
func RootItems(ctx api.Context) ([]api.Item, error) {
	return []api.Item{{ID: "root", Title: "Fallback root", QueryFallback: true}}, nil
}
