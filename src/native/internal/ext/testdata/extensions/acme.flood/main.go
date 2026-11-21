package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

// floodItems is far more rows than a palette can show, and its encoding is far larger than one protocol line
// may carry, so the host must fail the call rather than read a partial answer or render every row.
const floodItems = 100000

// SearchItems answers every query with floodItems rows.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	var items []api.Item
	for i := 0; i < floodItems; i++ {
		items = append(items, api.Item{ID: "flood", Title: "flood row"})
	}
	return items, nil
}
