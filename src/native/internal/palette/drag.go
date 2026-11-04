package palette

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/search"
)

// dragSourceOf returns the files a result drags out, and false when it has none or the host offers no drag.
func (p *Palette) dragSourceOf(it search.Item) (DragSource, bool) {
	if len(it.DragPaths) == 0 || p.opts.DragRow == nil {
		return DragSource{}, false
	}
	return DragSource{Paths: it.DragPaths, Image: it.DragImage}, true
}

// dragRow draws result i through the host's drag element. That element takes the press, so it is the click target
// too: a click chooses result i and a double click runs it, as an undragged row does. A press that moves drags and
// is neither.
func (p *Palette) dragRow(c *ui.Context, i int, src DragSource, draw func()) {
	target := p.opts.DragRow(c, src, draw)
	if target.Clicked() {
		p.choose(i)
	}
	if target.DoubleClicked() {
		p.choose(i)
		p.runDefault()
	}
}
