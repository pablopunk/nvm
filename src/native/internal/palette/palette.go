package palette

import (
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
	Size() (width, height int)
	SetPosition(x, y int)
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
	// DragRow draws a row whose item carries DragPaths: the host wraps draw so that dragging the row offers the files
	// to other applications. Nil draws the row as it is. The palette itself does not import the OS transfer package.
	DragRow func(c *ui.Context, paths []string, draw func())
}

// Palette is the shell: one window, a stack of views, the search of the view on top, and the action menu.
type Palette struct {
	opts Options
	win  Window

	stack   []*View
	query   string
	results []search.Item
	chosen  int
	list    ui.ListState
	input   ui.Handle

	actionsOpen bool
	actChosen   int
	actList     ui.ListState

	toastMsg string
	notice   *Notice
	// returnTo is the window the keyboard goes back to when the palette hides, captured when it was shown.
	returnTo *platform.FocusTarget
	// shows counts the times the palette was shown, so a delayed dismissal can tell it was shown again.
	shows int
	// announcedNotice is the alert assistive technology was last told about.
	announcedNotice *Notice
	phIndex         int
	// settled is false while the providers of the shown query are still answering: the list then shows the
	// rows it has, and no empty state.
	settled    bool
	shownView  *View
	shownQuery string
	// pushes counts the views pushed, so an action can tell whether it opened one.
	pushes int
	// announced is the selection assistive technology was last told about.
	announced selection
}

// New returns a palette whose stack holds only the root. Attach gives it its window before it is shown.
func New(opts Options) *Palette {
	return &Palette{opts: opts, stack: []*View{opts.Root}}
}

// Attach gives the palette the window it draws into and shows.
func (p *Palette) Attach(w Window) { p.win = w }

// SetStack replaces the navigation stack without leaving the views on it. The host uses it to start from a
// given state, and the goldens use it to render one.
func (p *Palette) SetStack(views ...*View) {
	p.stack = append([]*View(nil), views...)
	p.shownView = nil
}

func (p *Palette) cur() *View { return p.stack[len(p.stack)-1] }

// Show shows the palette on the first level with every search reset, then runs OnShow.
func (p *Palette) Show() {
	if !p.win.IsVisible() {
		p.captureFocus()
	}
	p.shows++
	if p.opts.Position != nil {
		w, h := p.win.Size()
		x, y := p.opts.Position(w, h)
		p.win.SetPosition(x, y)
	}
	p.phIndex++
	for len(p.stack) > 1 {
		p.leave(p.cur())
		p.stack = p.stack[:len(p.stack)-1]
	}
	p.query = ""
	p.actionsOpen = false
	p.shownView = nil
	p.resetRootHistory()
	p.search()
	if p.opts.OnShow != nil {
		p.opts.OnShow()
	}
	p.input.Focus()
	p.win.Show()
}

// Toggle shows the palette when it is hidden and hides it when it is shown: the hotkey's action.
func (p *Palette) Toggle() {
	if p.win.IsVisible() {
		p.Hide()
		return
	}
	p.Show()
}

// Open shows the palette with v pushed above the root, as the tray and the hotkey's sub-commands do.
func (p *Palette) Open(v *View) {
	p.Show()
	p.push(v)
}

// Hide closes the palette window and runs OnHide.
func (p *Palette) Hide() {
	p.win.Hide()
	p.returnFocus()
	if p.opts.OnHide != nil {
		p.opts.OnHide()
	}
}

// Toast sets the one-line message shown under the search field on the next frame.
func (p *Palette) Toast(msg string) { p.toastMsg = msg }

// Refresh runs the current search again and keeps the cursor where it is. It is safe from any goroutine.
func (p *Palette) Refresh() {
	p.win.Update(func() {
		keep := p.chosen
		p.invalidateRows()
		p.search()
		p.choose(keep)
	})
}

// ReloadRows forgets the rows of every view and searches again, from the UI thread: the host calls it when
// the data behind the root changed (applications, extensions).
func (p *Palette) ReloadRows() {
	p.invalidateRows()
	p.shownView = nil
	p.search()
}

// Push opens a view above the current one. It implements search.Host, so the argument is a *View.
func (p *Palette) Push(v any) {
	view, ok := v.(*View)
	if !ok {
		panic("palette: Push wants a *View")
	}
	p.push(view)
}

func (p *Palette) push(v *View) {
	p.stack = append(p.stack, v)
	p.pushes++
	p.query = ""
	p.actionsOpen = false
	p.search()
	p.input.Focus()
}

// Pop returns to the view below the current one. At the root it does nothing.
func (p *Palette) Pop() {
	if len(p.stack) < 2 {
		return
	}
	p.leave(p.cur())
	p.stack = p.stack[:len(p.stack)-1]
	p.query = ""
	p.actionsOpen = false
	p.search()
	p.input.Focus()
}

// leave runs a view's OnLeave and stops its searches: nothing it returns is shown again.
func (p *Palette) leave(v *View) {
	if f := v.OnLeave; f != nil {
		f()
	}
	if v.coord != nil {
		v.coord.Cancel()
	}
}

// Query returns the text of the search field (search.Host).
func (p *Palette) Query() string { return p.query }

// SetQuery replaces the text of the search field and searches again (search.Host).
func (p *Palette) SetQuery(q string) {
	p.query = q
	p.search()
}

// Choose moves the cursor to row i, clamped to the rows that exist.
func (p *Palette) Choose(i int) { p.choose(i) }

// OpenActionMenu opens the action menu of the chosen row with the action at index chosen selected.
func (p *Palette) OpenActionMenu(chosen int) {
	p.actionsOpen, p.actChosen = true, chosen
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
	p.chosen = max(0, min(i, len(p.results)-1))
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
