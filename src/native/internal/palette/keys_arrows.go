package palette

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// searchEdge is an arrow the search field let go of at its edge, where the caret cannot move any further, or an
// arrow a grid takes from the field.
type searchEdge int

const (
	edgeNone searchEdge = iota
	edgeLeft
	edgeRight
	edgeCellBack
	edgeCellForward
)

// onSearchKey sees ArrowLeft and ArrowRight before the search field takes them. A grid takes them for its cells. In a
// list, inside the query the field moves the caret, as in 1.x; at the start ArrowLeft closes an open menu or goes back
// a page, and at the end ArrowRight opens the menu of the chosen row. The span is the one the last frame drew, so it is
// trusted only while the text is the same: typed text the frame has not built yet sends the arrow to the field.
func (p *Palette) onSearchKey(k ui.TextKey) bool {
	if k.Mods != 0 || k.Text() != p.query {
		return false
	}
	start := p.searchSpan[0]
	switch {
	case p.gridTakesArrow(k.Key):
		p.edge = gridEdge(k.Key)
	case k.Key == ui.KeyLeft && start == 0 && p.leftLeavesSearch():
		p.edge = edgeLeft
	case k.Key == ui.KeyRight && start == utf8.RuneCountInString(p.query) && p.rightOpensMenu():
		p.edge = edgeRight
	default:
		return false
	}
	return true
}

// gridTakesArrow reports whether a grid takes ArrowLeft and ArrowRight from the field, as 1.x does: the cells move and
// the caret stays put. Without rows, or with the action menu open, the field keeps them.
func (p *Palette) gridTakesArrow(key ui.Key) bool {
	return (key == ui.KeyLeft || key == ui.KeyRight) && p.cur().Grid != nil && !p.actionsOpen && len(p.results) > 0
}

// gridEdge names the cell step an arrow a grid takes asks for.
func gridEdge(key ui.Key) searchEdge {
	if key == ui.KeyLeft {
		return edgeCellBack
	}
	return edgeCellForward
}

// leftLeavesSearch reports whether ArrowLeft at the start of the query does something: it closes the open menu, or goes
// back from a page.
func (p *Palette) leftLeavesSearch() bool {
	return p.actionsOpen || (len(p.stack) > 1 && p.listsRows())
}

// rightOpensMenu reports whether ArrowRight at the end of the query opens the menu of the chosen row.
func (p *Palette) rightOpensMenu() bool {
	return !p.actionsOpen && p.listsRows() && len(p.menuActions()) > 0
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
	case edge == edgeCellBack:
		p.choose(p.chosen - 1)
	case edge == edgeCellForward:
		p.choose(p.chosen + 1)
	case edge == edgeRight:
		p.actionsOpen, p.actChosen = true, 0
	case p.actionsOpen:
		p.actionsOpen = false
	default:
		p.Pop()
	}
}
