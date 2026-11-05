package palette

import "github.com/pablopunk/nvm/native/internal/search"

// OpenAction presents a shortcut's destination before running it. An asynchronous command gets a target-specific
// loading view which its result replaces, keeping the root out of the opening transition and loading out of history.
func (p *Palette) OpenAction(title string, action search.Action) {
	p.prepareShow()
	p.leaveUnsurvivingPages()
	covered := p.cur()
	p.entryBoundary = len(p.stack)
	opening := &View{Title: title, Placeholder: title, Loading: func() bool { return p.actionPending }}
	p.stack = append(p.stack, opening)
	p.openingView = opening
	p.actionPending = true
	sequence := p.actionSequence
	p.syncActive(covered)
	p.present()
	p.runAction(action)
	if sequence == p.actionSequence {
		p.actionPending = false
		p.search()
	}
}
