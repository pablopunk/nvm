package palette

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/search"
)

// activation is one run of an action of a row that carries a learn key. It is learned once, when the run succeeds.
type activation struct {
	key       string
	query     string
	concealed bool
	async     bool // the action ended its run later, through BeginActionOutcome, which then reports the outcome
	learned   bool
}

// runRowAction runs an action of the row whose learn key is key. Every way a user activates a row's action (Enter, a
// click, the action menu and the action's shortcut) comes here, so the usage ranking has one learning point: the
// row is recorded with the query when the action succeeds, once, and never when it fails, is cancelled or is
// superseded. An action that ends later reports that through BeginActionOutcome; any other counts as succeeded when
// its run returns.
func (p *Palette) runRowAction(key string, a search.Action) {
	outer := p.activation
	act := &activation{key: key, query: strings.TrimSpace(p.query), concealed: p.cur().Concealed}
	p.activation = act
	defer func() { p.activation = outer }()
	p.runAction(a)
	if !act.async {
		p.learn(act)
	}
}

// learn records the row of act with the query it was activated for, unless it was learned already, has no key, is on a
// concealed view or the palette learns nothing.
func (p *Palette) learn(act *activation) {
	if act.learned {
		return
	}
	act.learned = true
	if act.key != "" && p.opts.Learner != nil && !act.concealed {
		p.opts.Learner.Record(act.key, act.query)
	}
}
