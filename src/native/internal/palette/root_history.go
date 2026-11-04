package palette

// The root's query history is the only history: a sub-view has no walk, and a walk ends when the view changes.
// The methods below are the palette's side of it; the key routing calls them in the order keys() fixes.

// rootHistory returns the query history when the root is the view on top, and nil otherwise.
func (p *Palette) rootHistory() *QueryHistory {
	if p.opts.History == nil || len(p.stack) != 1 {
		return nil
	}
	return p.opts.History
}

// rememberQuery records a query submitted at the root. A concealed view's query is never recorded.
func (p *Palette) rememberQuery(query string) {
	if p.cur().Concealed {
		return
	}
	if h := p.rootHistory(); h != nil {
		h.Remember(query)
	}
}

// resetRootHistory ends a walk through the history, whatever view is on top.
func (p *Palette) resetRootHistory() {
	if p.opts.History != nil {
		p.opts.History.Reset()
	}
}

// rootSelectionIsAtTop reports whether ArrowUp has no row above it to move to: the first row is chosen, or
// there are no rows. At the top, ArrowUp recalls the previous query instead.
func (p *Palette) rootSelectionIsAtTop() bool {
	return len(p.results) == 0 || p.chosen == 0
}

// stepRootHistory walks the history with the arrow keys: ArrowUp at the top of the list recalls the previous
// query, and ArrowDown during a walk recalls the next one. up and down are the arrow keys pressed this frame,
// read once by the caller (reading a key consumes it). It reports whether the history used the key.
func (p *Palette) stepRootHistory(up, down bool) bool {
	h := p.rootHistory()
	if h == nil {
		return false
	}
	switch {
	case up && p.rootSelectionIsAtTop():
		recalled, ok := h.Older(p.query)
		if ok {
			p.recallQuery(recalled)
		}
		return ok
	case down && h.Navigating():
		recalled, _ := h.Newer()
		p.recallQuery(recalled)
		return true
	}
	return false
}

// restoreRootDraft is Escape during a walk: the typed text comes back and the walk ends. It reports whether a
// walk was under way, in which case Escape is used up and the palette stays open.
func (p *Palette) restoreRootDraft() bool {
	h := p.rootHistory()
	if h == nil || !h.Navigating() {
		return false
	}
	typed, _ := h.Restore()
	p.recallQuery(typed)
	return true
}

// recallQuery puts a query on the search field and searches it.
func (p *Palette) recallQuery(query string) {
	p.replaceQuery(query)
	p.search()
	p.input.Focus()
}
