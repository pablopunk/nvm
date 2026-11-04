package app

import "github.com/pablopunk/nvm/native/internal/search"

// rootSource is one provider of the root: its ID and the function that computes its rows.
type rootSource struct {
	id   string
	rows func(p *App, in *rootInput, q string) []search.Item
	// provider, when set, replaces rows: the source streams its results in batches.
	provider func(p *App) search.Provider
}

// rootSources is the root's provider list. The coordinator merges the rows by score, and rows of equal score
// keep the order of this list, so a new source goes where its rows should sort among equals. Adding a
// provider is one line here and one row function in providers_rows.go.
var rootSources = []rootSource{
	{id: "calculator", rows: calculatorRows},
	{id: "timer", rows: timerRows},
	{id: "quicklinks", rows: quicklinkRows},
	{id: "snippets", rows: snippetRows},
	{id: "extensions", provider: extensionProvider},
	{id: "commands", rows: commandRows},
	{id: "guide", rows: guideRootRows},
	{id: "applications", rows: applicationRows},
	{id: "files", provider: rootFileProvider},
	{id: "clipboard", rows: clipboardRows},
	{id: "conversations", rows: conversationRows},
	{id: "web", rows: webRows},
}
