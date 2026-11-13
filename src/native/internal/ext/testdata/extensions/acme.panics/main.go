package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

// SearchItems panics on the calling goroutine for one query. The child recovers the panic, reports it, and keeps
// running.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	if query == "boom" {
		panic("fixture panic on query boom")
	}
	return []api.Item{{ID: "ok", Title: "Still alive"}}, nil
}
