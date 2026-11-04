package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

// Package initialisation loops forever, so the child never reports ready.
func init() {
	for {
	}
}

func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	return nil, nil
}
