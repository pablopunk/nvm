package main

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// SearchItems answers after a delay longer than the test's hard budget. It declares no RootItems.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	time.Sleep(300 * time.Millisecond)
	return []api.Item{{ID: "late", Title: "Too late"}}, nil
}
