package api

// ExtensionRecord is one installed extension as Extensions.List reports it. Built-in extensions are always on and are
// listed only when they have a menu bar item. MenuBar is true for a running extension with a menu bar item, and
// MenuBarShown says whether the user shows it.
type ExtensionRecord struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Subtitle     string       `json:"subtitle,omitempty"`
	Enabled      bool         `json:"enabled"`
	BuiltIn      bool         `json:"builtIn,omitempty"`
	MenuBar      bool         `json:"menuBar,omitempty"`
	MenuBarShown bool         `json:"menuBarShown,omitempty"`
	Capabilities []Capability `json:"capabilities,omitempty"`
}

// Extensions lists the installed extensions and turns them off. List needs the extensions declaration; the other
// methods need extensions.write and run only from a palette action. Nothing enables an extension: OpenReview shows the
// capability review of a disabled extension in the palette and returns at once, and only the user enables it there.
// Built-ins and the calling extension itself are refused.
type Extensions interface {
	List() []ExtensionRecord
	Disable(id string) error
	OpenReview(id string) error
	// SetMenuBarItem shows or hides the menu bar item of a running extension that has one.
	SetMenuBarItem(id string, shown bool) error
}
