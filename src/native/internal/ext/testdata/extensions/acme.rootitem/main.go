package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

// RootItems is the one root row of this fixture: a daily note whose primary action copies its text, and whose menu also
// offers Copy Text. It is the extension root item the Command-K menu test checks.
func RootItems(ctx api.Context) ([]api.Item, error) {
	return []api.Item{{
		ID:       "daily",
		Title:    "Daily note",
		Subtitle: "Today's note",
		Primary:  &api.Action{Kind: api.ActionCopyText, Title: "Copy note", Text: "Daily note"},
		Actions:  []api.Action{{Kind: api.ActionCopyText, Title: "Copy Text", Text: "Daily note"}},
	}}, nil
}
