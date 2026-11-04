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
	KindWindows       = "Window"
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
	// IconFallback is the lucide icon drawn while IconPath has no system icon, or when the system has none.
	IconFallback string
	// DragPaths are files the row drags out to other applications. Empty means the row does not drag.
	DragPaths []string
	Image     *ui.Bitmap
	Accessory string
	Actions   []Action
	Detail    func(c *ui.Context, h Host) // preview pane
	ID        string
	// Disabled rows stay visible as context: they draw muted, carry no actions and are skipped by the keyboard.
	Disabled bool
	// Contextual rows are plain guidance text, such as a load error, drawn full contrast and wrapped.
	Contextual bool
	// Score ranks the row against the others of its query: the merged results are sorted by it, highest
	// first, with ties kept in provider order.
	Score int
	// LearnKey names the row to the usage learner. The coordinator raises Score by what the user chose for
	// the key before, and the palette records the key when the row's primary action runs. Empty means the
	// row is not learned.
	LearnKey string
}
