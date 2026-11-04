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
	// toastsStale is set when the palette hid or showed: the window's toasts of the last presentation are closed
	// on the next frame, because the window freezes their timers while it is hidden.
	toastsStale bool
	notice      *Notice
	// returnTo is the window the keyboard goes back to when the palette hides, captured when it was shown.
	returnTo *platform.FocusTarget
	// summoned is the window that had the keyboard before the palette was last shown. Unlike returnTo it survives the
	// hide, because an action that chooses where the keyboard goes clears returnTo first.
	summoned *platform.FocusTarget
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

// Show shows the palette with every search reset, on the root or on the chat page that survived the last hide, then
// runs OnShow.
func (p *Palette) Show() {
	if !p.win.IsVisible() || !p.win.IsFocused() {
		p.captureFocus()
	}
	p.shows++
	p.blurred = false
	if p.opts.Position != nil {
		w, h := p.win.Size()
		x, y := p.opts.Position(w, h)
		p.win.SetPosition(x, y)
	}
	p.phIndex++
	p.query = ""
	p.actionsOpen = false
	p.shownView = nil
	p.resetRootHistory()
	p.dropToasts()
	p.leaveUnsurvivingPages()
	p.search()
	if p.opts.OnShow != nil {
		p.opts.OnShow()
	}
	p.input.Focus()
	p.win.Show()
	p.syncActive(nil)
}

// Toggle is the summon shortcut's action, as 1.x reads it: a palette that is shown and has the keyboard hides. Any other
// state shows it, so a shown palette behind another window comes forward. A focus loss still within its grace counts as
// the keyboard being here, because the shortcut's own press causes that loss.
func (p *Palette) Toggle() {
	if p.win.IsVisible() && (p.blurred || p.win.IsFocused()) {
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

// Hide closes the palette window and runs OnHide. The stack stays as it is until the next Show.
func (p *Palette) Hide() {
	p.dropToasts()
	for _, v := range p.stack {
		if v.OnHide != nil {
			v.OnHide()
		}
	}
	p.win.Hide()
	p.syncActive(nil)
	p.returnFocus()
	if p.opts.OnHide != nil {
		p.opts.OnHide()
	}
}

// dropToasts forgets the message and the window toasts of the presentation that is ending.
func (p *Palette) dropToasts() {
	p.toastMsg = ""
	p.toastsStale = true
}

// Refresh runs the current search again and keeps the cursor where it is. It is safe from any goroutine.
func (p *Palette) Refresh() {
	p.win.Update(func() {
		keep := p.chosen
		p.invalidateRows()
		p.search()
		p.choose(keep)
	})
}

// RefreshTop runs the search again when the top view satisfies pred and the palette is shown. It is safe from any
// goroutine, so state that changes in the background redraws an open view without the user reopening it.
func (p *Palette) RefreshTop(pred func(*View) bool) {
	p.win.Update(func() {
		if !p.win.IsVisible() || !pred(p.cur()) {
			return
		}
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
	covered := p.cur()
	p.stack = append(p.stack, v)
	p.pushes++
	p.query = ""
	p.actionsOpen = false
	p.syncActive(covered)
	p.search()
	p.input.Focus()
}

// Visible reports whether the palette window is shown.
func (p *Palette) Visible() bool { return p.win.IsVisible() }

// PopIf goes back from v when v is the page on top, and reports whether it did. It never removes the root.
func (p *Palette) PopIf(v *View) bool {
	if len(p.stack) < 2 || p.cur() != v {
		return false
	}
	p.Pop()
	return true
}

// Pop returns to the view below the current one. At the root it does nothing.
func (p *Palette) Root() {
	for len(p.stack) > 1 {
		p.Pop()
	}
}

func (p *Palette) Pop() {
	if len(p.stack) < 2 {
		return
	}
	p.leave(p.cur())
	p.stack = p.stack[:len(p.stack)-1]
	p.query = ""
	p.actionsOpen = false
	p.syncActive(nil)
	p.search()
	p.input.Focus()
}

// leaveUnsurvivingPages pops the pages above the topmost page that survives a hide, down to the root when none does.
// It runs when the palette shows, as Electron's onShown does: Electron's hidePanes keeps the last live AI chat the same
// way and clears the rest.
func (p *Palette) leaveUnsurvivingPages() {
	for len(p.stack) > 1 && !p.cur().SurvivesHide {
		p.leave(p.cur())
		p.stack = p.stack[:len(p.stack)-1]
	}
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
