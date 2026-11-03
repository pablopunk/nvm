package app

import "github.com/pablopunk/nvm/native/internal/search"

// rootSource is one provider of the root: its ID and the function that computes its rows.
type rootSource struct {
	id   string
	rows func(p *Palette, in *rootInput, q string) []search.Item
}

// rootSources is the root's provider list. The coordinator merges the rows by score, and rows of equal score
// keep the order of this list, so a new source goes where its rows should sort among equals. Adding a
// provider is one line here and one row function in providers_rows.go.
var rootSources = []rootSource{
	{"calculator", calculatorRows},
	{"timer", timerRows},
	{"quicklinks", quicklinkRows},
	{"snippets", snippetRows},
	{"extensions", extensionRows},
	{"commands", commandRows},
	{"applications", applicationRows},
	{"files", fileRows},
	{"clipboard", clipboardRows},
	{"conversations", conversationRows},
	{"web", webRows},
}
