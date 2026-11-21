package main

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// lateAnswer is how long acme.late takes to answer a search: longer than the palette's local hard budget, which cuts
// the call off, and shorter than the test manager's search budget, which would kill the child.
const lateAnswer = 200 * time.Millisecond

// Commands declares one command, which the palette lists without waiting for SearchItems.
func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{ID: "late", Title: "Fixture late command"}}, nil
}

// SearchItems answers late. Its row must still reach the palette once it answers.
func SearchItems(ctx api.Context, query string) ([]api.Item, error) {
	time.Sleep(lateAnswer)
	return []api.Item{{ID: "late-row", Title: "Fixture late row"}}, nil
}
