package palette

import "github.com/pablopunk/nvm/native/internal/search"

// hasDefaultAction reports whether Enter does something on the row: menu-only actions never run on Enter.
func hasDefaultAction(it search.Item) bool {
	_, ok := it.DefaultAction()
	return ok
}
