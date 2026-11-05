package main

import "github.com/pablopunk/nvm/native/internal/ext/api"

// RootItems is the one root row of this fixture: a daily note with text to preview and no preview action of its own. The
// host adds Preview to its menu, as 1.x does for a row with text. It is the extension row the Command-K menu test checks.
func RootItems(ctx api.Context) ([]api.Item, error) {
	return []api.Item{{
		ID:       "daily",
		Title:    "Daily note",
		Subtitle: "Today's note",
		Primary:  &api.Action{Kind: api.ActionCopyText, Title: "Copy note", Text: "Daily note"},
		Detail:   &api.Detail{Text: "Buy milk and bread."},
	}}, nil
}
