package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

// SearchItems allocates far more memory than the child's address-space limit allows.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	block := make([]byte, 4<<30)
	block[len(block)-1] = 1
	return []api.Item{{ID: "big", Title: string(block[len(block)-1:])}}, nil
}
