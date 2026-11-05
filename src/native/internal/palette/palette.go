package palette

import (
	"sync/atomic"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/search"
)

// Window is the window the palette draws into and shows. The host's window implements it.
type Window interface {
	// Update runs fn on the UI thread, then redraws. It is safe from any goroutine.
	Update(fn func())
	Show()
	Hide()
	IsVisible() bool
	// IsFocused reports whether the window has the keyboard. A visible window without it is one another window took the
	// keyboard from, which the summon shortcut brings forward rather than hides.
	IsFocused() bool
	Size() (width, height int)
	SetPosition(x, y int)
	// Focus gives the window the keyboard.
	Focus()
}

// Options are what the shell needs from its host.
type Options struct {
	// Root is the first level of the stack.
	Root *View
	// Clock measures the search budgets of the view coordinators.
	Clock platform.Clock
	// Synchronous runs the providers in the calling goroutine, so a query's rows are complete when it returns.
	// The goldens and the benchmarks use it; the app does not.
	Synchronous bool
	// Position returns where the window's top left corner goes for a window of w by h.
	Position func(w, h int) (x, y int)
	// Icon returns the decoded system icon of a file, or nil when it has none.
	Icon func(path string) *ui.Bitmap
	// AskAI returns the view that Tab opens at the root for the query. Nil disables Tab.
	AskAI func(q string) *View
	// OnShow runs each time the palette is shown, after its state is reset; the host loads what its rows need.
	OnShow func()
	// OnPresented runs after a newly shown window reveals its fresh content frame.
	OnPresented func()
	// OnHide runs each time the palette is hidden, by any route; the host stops what only a visible palette needs.
	// It may run more than once per show, so it must be idempotent.
	OnHide func()
	// History remembers the queries submitted at the root and recalls them with ArrowUp and ArrowDown. Nil
	// disables it.
	History *QueryHistory
	// Learner ranks and records the rows that carry a LearnKey. Nil disables learning.
	Learner search.Learner
	// Focus gives the keyboard back to the window that had it when the palette was shown, when the palette hides
	// without an action that chose its own destination. Nil disables it.
	Focus platform.Focus
	// Feedback shows a message raised while the palette is hidden, where it outlives the hide, as 1.x's HUD does. Nil
	// drops such messages.
	Feedback func(f Feedback)
	// DragRow draws a row or grid cell whose item carries DragPaths: the host wraps draw in an element that a press
	// dragging it turns into a native drag of the files, and returns that element. The palette takes the element as
	// the row's click target. Nil draws the row as it is. The palette itself does not import the OS transfer package.
	DragRow func(c *ui.Context, src DragSource, draw func()) ui.Element
	// Resize sizes the window for a presentation when the top view asks for another size (see desiredSize). Nil keeps
	// the window's size.
	Resize func(width, height int)
	// Layout is how the palette sits in its window; the zero value is the launcher panel.
	Layout WindowLayout
}

// DragSource is the files a row drags out: their paths, and the file whose picture the drag shows. An empty Image
// means the first path.
type DragSource struct {
	Paths []string
	Image string
}

// Palette is the shell: one window, a stack of views, the search of the view on top, and the action menu.
type Palette struct {
	actionSequence uint64
	actionPending  bool
	openingView    *View
	actionQuery    string
	opts           Options
	win            Window
	// refresh holds the queries background sources asked the palette to search again.
	refresh searchRefresher

	stack []*View
	// entryBoundary is the first view opened by an independent shortcut. Escape at that view
	// returns focus instead of exposing the root underneath it.
	entryBoundary int
	query         string
	// caretToEnd is set when the query was replaced: the input takes the caret to the end of the text once.
	caretToEnd   bool
	results      []search.Item
	chosen       int
	choicePinned bool
	list         ui.ListState
	listChoice   ui.Selection[string]
	input        ui.Handle
	interact     interactionState // pointer and click state of rows and cells, see interact.go
	// focusList is set while the view on top hides its field and its list must take the keyboard when it is built.
	focusList bool

	actionsOpen     bool
	actChosen       int
	menuQuery       string
	menuSnapshot    []search.Action
	menuOwner       string
	menuRanked      []search.Action
	menuRankedQuery string
	menuInput       ui.Handle
	menuScroll      ui.ScrollState
	menuReveal      bool
	menuPointer     pointerAt

	notice *Notice
	// returnTo is the window the keyboard goes back to when the palette hides, captured when it was shown.
	returnTo *platform.FocusTarget
	// summoned is the window that had the keyboard before the palette was last shown. Unlike returnTo it survives the
	// hide, because an action that chooses where the keyboard goes clears returnTo first.
	summoned *platform.FocusTarget
	// shows counts the times the palette was shown, so a delayed dismissal can tell it was shown again.
	shows         int
	revealFrame   int
	revealWaiting bool
	// announcedNotice is the alert assistive technology was last told about.
	announcedNotice *Notice
	phIndex         int
	// settled is false while the providers of the shown query are still answering: the list then shows the
	// rows it has, and no empty state. pending is true while a provider reports a call in flight (View.Pending
	// covers the views that are not provider-backed).
	settled    bool
	pending    bool
	shownView  *View
	shownQuery string
	// pushes counts the views pushed, so an action can tell whether it opened one.
	pushes int
	// running is the title of the action that is running, which titles the toasts it raises.
	running string
	// announced is the selection assistive technology was last told about.
	announced selection
	// searchSpan is the search field's selection as the last frame drew it, and edge the arrow the field let go of
	// at its end, which the next frame acts on (keys_arrows.go).
	searchSpan [2]int
	edge       searchEdge
	// blurred is set from a focus loss until its grace passes (blur.go). dialogs counts the native dialogs that have the
	// keyboard for the palette (dialog.go); the palette stays open while one does.
	blurred bool
	dialogs atomic.Int32
	// windowWidth and windowHeight are the size the window was last resized to for a presentation (split.go).
	windowWidth, windowHeight int
}

// New returns a palette whose stack holds only the root. Attach gives it its window before it is shown.
func New(opts Options) *Palette {
	return &Palette{opts: opts, stack: []*View{opts.Root}}
}

// Attach gives the palette the window it draws into and shows.
func (p *Palette) Attach(w Window) { p.win = w }

// Query returns the text of the search field (search.Host).
func (p *Palette) Query() string { return p.query }

// SetQuery replaces the text of the search field and searches again (search.Host).
func (p *Palette) SetQuery(q string) {
	p.replaceQuery(q)
	p.search()
}

// replaceQuery puts text the palette did not type into the search field, with the caret after its last rune. The
// text is there to be continued, as a field's value is in Electron, so typing appends to it.
func (p *Palette) replaceQuery(q string) {
	p.query = q
	p.caretToEnd = true
}

// takeCaretToEnd reports whether the caret moves to the end of the search text in this frame, and clears the request.
func (p *Palette) takeCaretToEnd() bool {
	atEnd := p.caretToEnd
	p.caretToEnd = false
	return atEnd
}

// Choose moves the cursor to row i, clamped to the rows that exist.
func (p *Palette) Choose(i int) { p.choose(i) }

// OpenActionMenu opens the action menu of the chosen row with the action at index chosen selected.
func (p *Palette) OpenActionMenu(chosen int) {
	p.openActionMenu(chosen)
}

// Settled reports whether the rows on screen are the complete answer to the shown query.
func (p *Palette) Settled() bool { return p.settled }

// Top returns the view on top of the stack.
func (p *Palette) Top() *View { return p.cur() }

// Results returns the rows on screen.
func (p *Palette) Results() []search.Item { return p.results }

// Chosen returns the index of the row the cursor is on.
func (p *Palette) Chosen() int { return p.chosen }

func (p *Palette) choose(i int) {
	p.choicePinned = true
	p.clampChoice(i)
}

func (p *Palette) clampChoice(i int) {
	p.chosen = p.enabledNear(max(0, min(i, len(p.results)-1)), i)
	if len(p.results) > 0 {
		p.list.ScrollIntoView(p.rowOf(p.chosen))
	}
}

// rowOf returns the list row that holds row i: a grid puts Columns rows on each of its rows.
func (p *Palette) rowOf(i int) int {
	if g := p.cur().Grid; g != nil {
		return i / max(g.Columns, 1)
	}
	return i
}

// enabledNear returns the nearest row to i that the keyboard may choose, looking the way the choice moved (from the
// request). A list with no enabled row keeps i.
func (p *Palette) enabledNear(i, requested int) int {
	step := 1
	if requested < p.chosen {
		step = -1
	}
	for at := i; at >= 0 && at < len(p.results); at += step {
		if !p.results[at].Disabled {
			return at
		}
	}
	return i
}
