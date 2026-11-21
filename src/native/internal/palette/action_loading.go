package palette

// BeginAction marks a user action pending on the current surface. Completion cannot navigate or clear feedback for a
// newer action after the user has dismissed the palette, changed the query, or moved to another view.
func (p *Palette) BeginAction() func() bool {
	finish := p.BeginActionOutcome()
	return func() bool { return finish(true) }
}

// BeginActionOutcome is BeginAction for an action that ends with an outcome. A run that ends ok while it is still the
// latest action is the successful activation of its row, and the palette learns the row then, once.
func (p *Palette) BeginActionOutcome() func(ok bool) bool {
	act := p.activation
	if act != nil {
		act.async = true
	}
	p.actionSequence++
	sequence, view, query, shows := p.actionSequence, p.cur(), p.query, p.shows
	root := len(p.stack) == 1
	p.actionPending = true
	p.actionQuery = query
	p.requestActionFrame()
	completed := false
	return func(ok bool) bool {
		if completed || sequence != p.actionSequence {
			return false
		}
		completed = true
		if ok && act != nil {
			p.learn(act)
		}
		p.actionPending = false
		if view == p.openingView && view == p.cur() {
			p.search()
		}
		p.requestActionFrame()
		return view == p.cur() && (!root || query == p.query) && shows == p.shows
	}
}

// ActionCurrent is the probe of the action begun last: true until a newer or cancelled action supersedes it.
func (p *Palette) ActionCurrent() func() bool {
	sequence := p.actionSequence
	return func() bool { return sequence == p.actionSequence }
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
