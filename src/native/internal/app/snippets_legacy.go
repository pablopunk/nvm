package app

import (
	"github.com/pablopunk/nvm/native/internal/ext/builtins/snippets"
	"github.com/pablopunk/nvm/native/internal/store"
)

// legacySnippetsDocument is the snippets.json document the app kept in the user data store before the Snippets built-in.
type legacySnippetsDocument struct {
	Snippets []snippets.Snippet `json:"snippets"`
}

// loadLegacySnippets reads the snippets the app kept before the Snippets built-in. The built-in answers with them until
// its first save stores them in its own storage. An unreadable document holds none; the store has already set it aside.
func loadLegacySnippets(st *store.Store) []snippets.Snippet {
	doc := store.Doc(st, "snippets.json", 1, func() legacySnippetsDocument { return legacySnippetsDocument{} })
	loaded, err := doc.Load()
	if err != nil {
		return nil
	}
	return loaded.Snippets
}
