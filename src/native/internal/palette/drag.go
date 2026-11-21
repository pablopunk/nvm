package palette

import "github.com/pablopunk/nvm/native/internal/search"

// dragSourceOf returns the files a result drags out, and false when it has none or the host offers no drag.
func (p *Palette) dragSourceOf(it search.Item) (DragSource, bool) {
	if len(it.DragPaths) == 0 || p.opts.DragRow == nil {
		return DragSource{}, false
	}
	return DragSource{Paths: it.DragPaths, Image: it.DragImage}, true
}
