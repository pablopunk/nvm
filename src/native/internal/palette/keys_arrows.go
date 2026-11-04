package palette

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// searchEdge is an arrow the search field let go of at its edge, where the caret cannot move any further.
type searchEdge int

const (
	edgeNone searchEdge = iota
	edgeLeft
	edgeRight
)

// onSearchKey sees ArrowLeft and ArrowRight before the search field takes them. Inside the query the field moves the
// caret, as in 1.x. At the start ArrowLeft closes an open menu or goes back a page; at the end ArrowRight opens the
// menu of the chosen row. The span is the one the last frame drew, so it is trusted only while the text is the same:
// typed text the frame has not built yet sends the arrow to the field.
func (p *Palette) onSearchKey(k ui.TextKey) bool {
	if k.Mods != 0 || k.Text() != p.query {
		return false
	}
	start := p.searchSpan[0]
	switch {
	case k.Key == ui.KeyLeft && start == 0 && p.leftLeavesSearch():
		p.edge = edgeLeft
	case k.Key == ui.KeyRight && start == utf8.RuneCountInString(p.query) && p.rightOpensMenu():
		p.edge = edgeRight
	default:
		return false
	}
	return true
}

// leftLeavesSearch reports whether ArrowLeft at the start of the query does something: it closes the open menu, or goes
// back from a page.
func (p *Palette) leftLeavesSearch() bool {
	return p.actionsOpen || (len(p.stack) > 1 && p.listsRows())
}

// rightOpensMenu reports whether ArrowRight at the end of the query opens the menu of the chosen row.
func (p *Palette) rightOpensMenu() bool {
	return !p.actionsOpen && p.listsRows() && len(p.actions()) > 0
}

// listsRows reports whether the arrows act on the view's rows: a grid moves with them, and a body draws its own keys.
func (p *Palette) listsRows() bool {
	v := p.cur()
	return v.Grid == nil && v.Body == nil
}

// takeSearchEdge returns the arrow the field let go of, and forgets it.
func (p *Palette) takeSearchEdge() searchEdge {
	edge := p.edge
	p.edge = edgeNone
	return edge
}

// runSearchEdge does what the arrow asked for at the edge of the search field.
func (p *Palette) runSearchEdge(edge searchEdge) {
	switch {
	case edge == edgeRight:
		p.actionsOpen, p.actChosen = true, 0
	case p.actionsOpen:
		p.actionsOpen = false
	default:
		p.Pop()
	}
}
