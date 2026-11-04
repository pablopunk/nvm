package palette

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// Side is a page drawn beside the page that holds it, as 1.x's builder workspace draws its extension preview. It is a
// palette of its own over the window of the palette that holds it: its own pages, search or title, rows and action
// menu. While the side has the focus it takes the keys; Escape leaves its pages first, then gives the focus back.
type Side struct {
	root    *View
	shell   *Palette
	host    *Palette
	focused bool
}

// NewSide returns a side whose first page is root. It is drawn by the view that returns it from View.Side. SetRoot
// replaces the page, and Refresh searches the side's rows again after its data changed.
func NewSide(root *View) *Side {
	return &Side{root: root}
}

// Focused reports whether the side has the keyboard. A nil side has none.
func (s *Side) Focused() bool {
	return s != nil && s.focused
}

// Focus gives the keyboard to the side: its search field, or its list when its title hides the field.
func (s *Side) Focus() {
	if s == nil {
		return
	}
	s.focused = true
	if s.shell != nil {
		s.shell.focusInput()
	}
}

// Blur gives the keyboard back to the page that holds the side, its input.
func (s *Side) Blur() {
	if !s.Focused() {
		return
	}
	s.unfocus()
	if s.host != nil {
		s.host.focusInput()
	}
}

// unfocus clears the side's focus without moving the keyboard anywhere.
func (s *Side) unfocus() {
	if s != nil {
		s.focused = false
	}
}

// SetRoot replaces the side's first page and leaves every page above it.
func (s *Side) SetRoot(root *View) {
	s.root = root
	if s.shell != nil {
		s.shell.replaceRoot(root)
	}
}

// Refresh searches the side's rows again, keeping the cursor where it is. It is safe from any goroutine.
func (s *Side) Refresh() {
	if s != nil && s.shell != nil {
		s.shell.Refresh()
	}
}

// keys routes the keyboard to the side while it has the focus. Escape is the side's own: it pops a page, or gives the
// focus back at its first page, and never hides the palette.
func (s *Side) keys(c *ui.Context) {
	if c.Shortcut(0, ui.KeyEscape) && !s.shell.actionsOpen {
		if len(s.shell.stack) > 1 {
			s.shell.Pop()
		} else {
			s.Blur()
		}
		return
	}
	s.shell.keys(c, s.shell.cur())
}

// replaceRoot leaves every page and makes v the first page, searched again. It is how a side swaps its page.
func (p *Palette) replaceRoot(v *View) {
	for len(p.stack) > 1 {
		p.leave(p.cur())
		p.stack = p.stack[:len(p.stack)-1]
	}
	if p.stack[0] != v {
		p.leave(p.stack[0])
	}
	p.stack[0] = v
	p.query = ""
	p.actionsOpen = false
	p.shownView = nil
	p.search()
}

// bind creates the side's palette the first time the host draws it, over the host's options and window.
func (s *Side) bind(host *Palette) {
	if s.shell != nil {
		return
	}
	s.host = host
	s.shell = New(host.sideOptions(s.root))
	s.shell.Attach(sideWindow{host: host})
	s.shell.search()
	if s.focused {
		s.shell.focusInput()
	}
}

// draw draws the side's chrome and its page: the title or search row, the notices and the rows or body. The side is
// drawn on the surface the split gives it, which the host's pane does not paint.
func (s *Side) draw(c *ui.Context, host *Palette) {
	if s.root == nil {
		return
	}
	s.bind(host)
	k := design.Tokens(c)
	ui.Row(c).Grow(1).FillHeight().MinHeight(0).Clip().Background(k.Sunken).Children(func() {
		s.focusEdge(c)
		ui.Column(c).Grow(1).FillHeight().MinHeight(0).Clip().Children(func() {
			v := s.shell.cur()
			s.shell.header(c, v)
			ui.Column(c).Grow(1).MinHeight(0).Clip().Children(func() {
				if v.Body != nil {
					v.Body(c, s.shell)
					return
				}
				s.shell.listBody(c)
			})
		})
	})
}

// focusEdge marks the side that has the focus with a thin accent edge on its left, as 1.x's focused preview pane does.
func (s *Side) focusEdge(c *ui.Context) {
	color := ui.Transparent
	if s.focused {
		color = design.Tokens(c).AccentEmphasis
	}
	ui.Column(c).Width(2).FillHeight().Background(color).Children(func() {})
}

// sideWindow is the window of a side: the window of the palette that holds it. Its updates redraw that window, and its
// hide hides the palette, since a side has no window of its own.
type sideWindow struct {
	host *Palette
}

func (w sideWindow) Update(fn func())     { w.host.win.Update(fn) }
func (w sideWindow) Show()                {}
func (w sideWindow) Hide()                { w.host.Hide() }
func (w sideWindow) IsVisible() bool      { return w.host.win.IsVisible() }
func (w sideWindow) IsFocused() bool      { return w.host.win.IsFocused() }
func (w sideWindow) Focus()               {}
func (w sideWindow) Size() (int, int)     { return w.host.win.Size() }
func (w sideWindow) SetPosition(x, y int) {}

// sideOptions are the options of a side's palette: the host's clock, search mode, icons, drags and toasts, with none of
// the palette's own history, learning, placement, focus or show hooks.
func (p *Palette) sideOptions(root *View) Options {
	return Options{
		Root:        root,
		Clock:       p.opts.Clock,
		Synchronous: p.opts.Synchronous,
		Icon:        p.opts.Icon,
		DragRow:     p.opts.DragRow,
		Feedback:    p.opts.Feedback,
	}
}
