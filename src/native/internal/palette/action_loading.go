package palette

// BeginAction marks a user action pending on the current surface. Completion cannot navigate or clear feedback for a
// newer action after the user has dismissed the palette, changed the query, or moved to another view.
func (p *Palette) BeginAction() func() bool {
	p.actionSequence++
	sequence, view, query, shows := p.actionSequence, p.cur(), p.query, p.shows
	root := len(p.stack) == 1
	p.actionPending = true
	p.actionQuery = query
	p.requestActionFrame()
	completed := false
	return func() bool {
		if completed || sequence != p.actionSequence {
			return false
		}
		completed = true
		p.actionPending = false
		if view == p.openingView && view == p.cur() {
			p.search()
		}
		p.requestActionFrame()
		return view == p.cur() && (!root || query == p.query) && shows == p.shows
	}
}

func (p *Palette) cancelAction() {
	p.actionSequence++
	p.actionPending = false
	p.openingView = nil
}

func (p *Palette) requestActionFrame() {
	if p.win != nil {
		p.win.Update(func() {})
	}
}

func (p *Palette) loadingActionOrView() bool {
	v := p.cur()
	return p.actionPending || (v.Loading != nil && v.Loading()) || p.loadingMore()
}
