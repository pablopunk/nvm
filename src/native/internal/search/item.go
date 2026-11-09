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
	Title    string
	Subtitle string
	Mods     ui.Modifiers
	Key      ui.Key
	Label    string // shortcut as displayed, e.g. "⌃⌫"
	Run      func(h Host)
	Dismiss  Dismissal
	// Disabled names why the action cannot run now, as 1.x shows it under the title of a disabled menu row. A
	// disabled action stays in the menu, muted, and choosing it does nothing. Empty means the action runs.
	Disabled string
	// MenuHidden keeps the action out of the action menu while its shortcut still runs it, as an action panel hidden by
	// its extension does. The menu does not open on a row whose actions are all hidden.
	MenuHidden bool
	// Section is the title of the action menu group the action sits in, such as View. The menu draws it above the group's
	// first action. Empty means the action has no group.
	Section string
	// MenuOnly lists the action in the action menu without making it the row's default: Enter runs the first action that
	// is not MenuOnly, as 1.x's View actions never run on Enter.
	MenuOnly bool
}

// DefaultAction is the action Enter runs on the row: its first action the menu does not hold back as MenuOnly. It is false
// when the row has none.
func (i Item) DefaultAction() (Action, bool) {
	for _, action := range i.Actions {
		if !action.MenuOnly {
			return action, true
		}
	}
	return Action{}, false
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
	// DragImage is the file whose picture the drag shows, when it is not the first of DragPaths. Empty means the first.
	DragImage string
	Image     *ui.Bitmap
	Accessory string
	Actions   []Action
	Detail    func(c *ui.Context, h Host) // preview pane
	ID        string
	// Disabled rows stay visible as context: they draw muted, carry no actions and are skipped by the keyboard.
	Disabled bool
	// Contextual rows are plain guidance text, such as a load error, drawn full contrast and wrapped.
	Contextual bool
	// Answers marks a row that answers the query outright, such as a calculation. While one is in the merged result,
	// the rows marked QueryFallback are left out.
	Answers bool
	// QueryFallback marks an open-ended row that echoes the query, such as a web search or Automate. It is held back
	// until every provider has answered the first time, and it is left out when a row answers the query.
	QueryFallback bool
	// Score ranks the row against the others of its query: the merged results are sorted by it, highest
	// first, then by TiePriority, LastUsed and title (compareRows).
	Score int
	// LearnKey names the row to the usage learner. The coordinator raises Score by what the user chose for
	// the key before, and the palette records the key when the row's primary action runs. Empty means the
	// row is not learned.
	LearnKey string
	// LastUsed is the Unix milliseconds of the row's last use: the usage learner's for a LearnKey, or the extension's own
	// LastUsedMs. The root breaks score ties by it, most recent first, as 1.x does. Zero means never used.
	LastUsed int64
	// TiePriority orders rows of equal score and last use, higher first. 1.x gives builder chats -1, so they follow the
	// other rows.
	TiePriority int
}
