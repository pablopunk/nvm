package main

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// SearchItems returns a result far larger than one protocol line may carry.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	padding := strings.Repeat("x", 2<<20)
	return []api.Item{{ID: "huge", Title: padding}}, nil
}
