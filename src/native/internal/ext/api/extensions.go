package api

// ExtensionRecord is one installed extension. Built-ins, always on, are listed only when they have a menu bar item.
type ExtensionRecord struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Enabled      bool     `json:"enabled"`
	BuiltIn      bool     `json:"builtIn,omitempty"`
	HasMenuBar   bool     `json:"hasMenuBar,omitempty"`
	MenuBarShown bool     `json:"menuBarShown,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// Extensions lists extensions and turns them off. List needs the extensions capability; changes need extensions.write
// and a palette action, and refuse built-ins and the caller. OpenReview shows a disabled extension's review and returns
// at once; only the user enables the extension there.
type Extensions interface {
	List() []ExtensionRecord
	Disable(id string) error
	OpenReview(id string) error
	SetMenuBarItem(id string, shown bool) error
}
