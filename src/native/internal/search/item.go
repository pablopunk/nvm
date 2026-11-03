package search

import "github.com/egoist/mygo/ui"

// Kinds are the sources of rows. A Kind is the label at a row's right edge and the key that sections
// group by; extension rows use their extension's title instead.
const (
	KindCalculator    = "Calculator"
	KindTimer         = "Timer"
	KindQuicklinks    = "Quicklinks"
	KindSnippets      = "Snippets"
	KindCommands      = "Commands"
	KindApplications  = "Applications"
	KindFiles         = "Files"
	KindClipboard     = "Clipboard"
	KindConversations = "Conversations"
	KindWeb           = "Web"
	KindSearch        = "Search"
)

// Dismissal says whether the palette closes once an action has run.
type Dismissal int

const (
	// DismissAuto hides the palette after the action, unless the action pushed a view.
	DismissAuto Dismissal = iota
	// DismissNever leaves the palette open: the action changed data or the view stack in place.
	DismissNever
	// DismissAlways hides the palette after the action, even when the action pushed a view.
	DismissAlways
)

// Action is something you can do with a result: the first one runs on Enter, all are in the action menu.
type Action struct {
	Title   string
	Mods    ui.Modifiers
	Key     ui.Key
	Label   string // shortcut as displayed, e.g. "⌃⌫"
	Run     func(h Host)
	Dismiss Dismissal
}

// Item is one row in the palette.
type Item struct {
	Kind      string // label shown at the right edge: Application, Command, ...
	Title     string
	Subtitle  string
	Glyph     string // 1-2 char avatar when there is no icon
	Icon      string // lucide icon name
	Section   string // group header shown above the first row of a run
	Tone      string // yellow|blue|purple|green|red|orange|pink
	Prominent bool   // accent-tinted row (Automate)
	TintTitle bool   // title in the tone's color
	IconPath  string // file whose system icon is shown
	Image     *ui.Bitmap
	Accessory string
	Actions   []Action
	Detail    func(c *ui.Context, h Host) // preview pane
	ID        string
	// Score ranks the row against the others of its query: the merged results are sorted by it, highest
	// first, with ties kept in provider order.
	Score int
}
