package app

import "github.com/pablopunk/nvm/native/internal/search"

// rootSource is one provider of the root: its ID and the function that computes its rows.
type rootSource struct {
	id   string
	rows func(p *App, in *rootInput, q string) []search.Item
	// provider, when set, replaces rows: the source streams its results in batches.
	provider func(p *App, in *rootInput) search.Provider
}

// rootSources is the root's provider list. The coordinator merges the rows by score, then by tie priority, last use and
// title; only rows equal on all of those keep this order. Adding a provider is one line here and one row function in
// providers_rows.go.
var rootSources = []rootSource{
	{id: "extensions", provider: extensionProvider},
	{id: "commands", rows: commandRows},
	{id: "updates", rows: updateRootRows},
	{id: "clipboard", rows: clipboardRows},
	{id: "conversations", rows: conversationRows},
	{id: "web", rows: automateRows},
}
