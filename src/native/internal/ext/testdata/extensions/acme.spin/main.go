package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

// SearchItems never returns. The host must kill the child at its hard deadline.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	for {
	}
}

func RootItems(ctx api.Context) ([]api.Item, error) {
	return []api.Item{{ID: "root", Title: "Root still works after a restart"}}, nil
}
