package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

// SearchItems answers "learn" with two rows of equal score, each with its own learning key. The host ranks them by
// what the user chose for each key, so the row used most climbs above the other.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	return []api.Item{
		{ID: "alpha", Title: "Learn Alpha", LearnKey: "alpha", Primary: api.Primary(api.CopyText("Copy", "alpha", false))},
		{ID: "beta", Title: "Learn Beta", LearnKey: "beta", Primary: api.Primary(api.CopyText("Copy", "beta", false))},
	}, nil
}
