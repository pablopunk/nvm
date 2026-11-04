package palette

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/search"
)

// actions returns the actions of the chosen row, or of the page on top when it is a Body page with a menu, or nil.
func (p *Palette) actions() []search.Action {
	if it := p.chosenItem(); it != nil {
		return it.Actions
	}
	if v := p.cur(); v.Actions != nil {
		return v.Actions()
	}
	return nil
}

// chosenItem returns the chosen row, or nil on a Body view and without rows.
func (p *Palette) chosenItem() *search.Item {
	if p.cur().Body != nil || p.chosen < 0 || p.chosen >= len(p.results) {
		return nil
	}
	return &p.results[p.chosen]
}

// runDefault runs the first action of the chosen row: what Enter does. At the root the query is remembered
// first, and the row's learn key is recorded with the query after its action has run.
func (p *Palette) runDefault() {
	it := p.chosenItem()
	if it == nil || len(it.Actions) == 0 {
		return
	}
	query, key := strings.TrimSpace(p.query), it.LearnKey
	concealed := p.cur().Concealed
	p.rememberQuery(query)
	p.runAction(it.Actions[0])
	if key != "" && p.opts.Learner != nil && !concealed {
		p.opts.Learner.Record(key, query)
	}
}

// runAction runs an action and then applies its dismissal policy. Every way of running an action goes through
// here, so the policy holds for Enter, for the action menu and for a row's own shortcut.
func (p *Palette) runAction(a search.Action) {
	pushes := p.pushes
	a.Run(p)
	switch a.Dismiss {
	case search.DismissAlways:
		p.hideAfterAction()
	case search.DismissAuto:
		if p.pushes == pushes {
			p.hideAfterAction()
		}
	}
}

// onEnter is Enter in the input: run the action the menu has selected, submit the text of an input-driven
// view, or run the chosen row's first action.
func (p *Palette) onEnter() {
	v := p.cur()
	switch {
	case p.actionsOpen:
		acts := p.actions()
		p.actionsOpen = false
		if p.actChosen < len(acts) {
			p.runAction(acts[p.actChosen])
		}
	case v.Body != nil:
		if v.OnSubmit != nil {
			q := strings.TrimSpace(p.query)
			if !v.KeepQuery {
				p.query = ""
			}
			v.OnSubmit(q)
		}
	default:
		p.runDefault()
	}
}
