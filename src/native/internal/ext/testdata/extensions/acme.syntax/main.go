package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

// This source deliberately does not compile: the child must report it as a load failure, not crash.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	return undefinedHelper(query), nil
}
