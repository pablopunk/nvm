package palette

import (
	"strings"

	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/search"
)

// actions returns the actions of the chosen row, or of the page on top when it is a Body page with a menu, or nil.
func (p *Palette) actions() []search.Action {
	if it := p.actionableItem(); it != nil {
		return it.Actions
	}
	if v := p.cur(); v.Actions != nil {
		return v.Actions()
	}
	return nil
}

// menuActions are the actions the action menu lists: all of actions() but those marked MenuHidden, whose shortcuts still
// run through actions().
func (p *Palette) menuActions() []search.Action {
	return menuActionsFrom(p.actions())
}

func menuActionsFrom(all []search.Action) []search.Action {
	visible := make([]search.Action, 0, len(all))
	seen := make(map[string]bool)
	for _, a := range all {
		if !a.MenuHidden {
			if a.Label != "" {
				if seen[a.Label] {
					a.Label = ""
				} else {
					seen[a.Label] = true
				}
			}
			visible = append(visible, a)
		}
	}
	return visible
}

// shortcutActions are the actions whose keys run now: the chosen row's, then the page on top's own.
func (p *Palette) shortcutActions() []search.Action {
	var out []search.Action
	if it := p.actionableItem(); it != nil {
		out = append(out, it.Actions...)
	}
	if v := p.cur(); v.Actions != nil {
		out = append(out, v.Actions()...)
	}
	return out
}

// chosenItem returns the chosen row, or nil on a Body view and without rows.
func (p *Palette) chosenItem() *search.Item {
	if p.cur().Body != nil || p.chosen < 0 || p.chosen >= len(p.results) {
		return nil
	}
	return &p.results[p.chosen]
}

// actionableItem excludes retained rows belonging to a previous query or view.
func (p *Palette) actionableItem() *search.Item {
	if p.shownView != p.cur() || p.shownQuery != strings.TrimSpace(p.query) {
		return nil
	}
	return p.chosenItem()
}

// recordPresentedChoice records what was drawn, not a later provider update.
func (p *Palette) recordPresentedChoice() {
	p.presented = presentedChoice{}
	if p.shownView != p.cur() || p.shownQuery != strings.TrimSpace(p.query) {
		return
	}
	p.presented = presentedChoice{view: p.cur(), query: p.shownQuery, shows: p.shows}
	if item := p.chosenItem(); item != nil {
		action, hasAction := item.DefaultAction()
		p.presented = presentedChoice{view: p.cur(), query: p.shownQuery, id: item.ID, kind: item.Kind, title: item.Title, shows: p.shows, action: action, actions: append([]search.Action(nil), item.Actions...), hasAction: hasAction, learnKey: item.LearnKey}
	}
}

// admitPresentedChoice binds a key to the row last drawn unless the user explicitly moved the choice.
// A removed row is never replaced by another row under the same key press.
func (p *Palette) admitPresentedChoice() bool {
	shown := p.presented
	if p.choicePinned || shown.view == nil || shown.view != p.cur() || shown.query != strings.TrimSpace(p.query) || shown.shows != p.shows {
		return true
	}
	for i, row := range p.results {
		if row.Kind == shown.kind && ((shown.id != "" && row.ID == shown.id) || (shown.id == "" && row.ID == "" && row.Title == shown.title)) {
			p.chosen = i
			return true
		}
	}
	return false
}

// runDefault runs the first action of the chosen row: what Enter does. At the root the query is remembered
// first, and the row is learned by runRowAction when its action succeeds.
func (p *Palette) runDefault() {
	it := p.actionableItem()
	diag.ActionTrace("primary", logx.F{"has_item": it != nil, "menu_open": p.actionsOpen})
	if it == nil {
		return
	}
	action, ok := it.DefaultAction()
	if !ok {
		return
	}
	p.rememberQuery(strings.TrimSpace(p.query))
	p.runRowAction(it.LearnKey, action)
}

// RunAction runs an action the way choosing it in the palette does, dismissal policy included. A global shortcut
// runs its row's action through it.
func (p *Palette) RunAction(a search.Action) { p.runAction(a) }

// runAction runs an action and then applies its dismissal policy. Every way of running an action goes through
// here, so the policy holds for Enter, for the action menu and for a row's own shortcut.
func (p *Palette) runAction(a search.Action) {
	pushes, outer := p.pushes, p.running
	p.running = a.Title
	a.Run(p)
	p.running = outer
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
	q := strings.TrimSpace(p.query)
	diag.ActionTrace("enter", logx.F{"menu_open": p.actionsOpen, "same_view": p.shownView == v, "same_query": p.shownQuery == q, "body": v.Body != nil})
	unseen := v.Providers != nil && !p.choicePinned && (p.presented.view != v || p.presented.query != q || p.presented.shows != p.shows)
	if v.Body == nil && !p.actionsOpen && (p.shownView != v || p.shownQuery != q || unseen) {
		p.defaultEnter = &defaultEnterIntent{view: v, query: q, shows: p.shows}
		if p.shownView == v && p.shownQuery == q && v.Providers != nil {
			p.defaultEnter.gen = p.coordinator(v).Gen()
		} else {
			if v.Providers != nil {
				p.defaultEnter.gen = p.coordinator(v).Gen() + 1
			}
			p.search()
		}
		return
	}
	switch {
	case p.actionsOpen:
		p.runMenuAction(p.actChosen)
	case v.Body != nil:
		if v.OnSubmit != nil {
			q := strings.TrimSpace(p.query)
			if !v.KeepQuery {
				p.query = ""
			}
			v.OnSubmit(q)
		}
	default:
		pinned := p.choicePinned
		shown := p.presented
		if p.admitPresentedChoice() {
			if !pinned && shown.view == v && shown.query == q && shown.shows == p.shows {
				if shown.hasAction {
					p.rememberQuery(q)
					p.runRowAction(shown.learnKey, shown.action)
				}
			} else {
				p.runDefault()
			}
		}
	}
}

func (p *Palette) resolveDefaultEnter(v *View, q string, settled bool) {
	i := p.defaultEnter
	if i == nil || !settled || i.view != v || i.query != q || i.shows != p.shows || p.cur() != v || p.shownView != v || p.shownQuery != q || strings.TrimSpace(p.query) != q || !p.Visible() || p.actionsOpen || i.gen != 0 && (v.coord == nil || v.coord.Gen() != i.gen) {
		return
	}
	if p.presented.view != v || p.presented.query != q || p.presented.shows != p.shows {
		return
	}
	p.defaultEnter = nil
	if p.presented.hasAction {
		p.rememberQuery(q)
		p.runRowAction(p.presented.learnKey, p.presented.action)
	}
}

// runMenuAction closes the action menu and runs its action at index i, when there is one.
func (p *Palette) runMenuAction(i int) {
	acts := p.filteredMenuActions()
	if i < 0 || i >= len(acts) || acts[i].Disabled != "" || acts[i].Run == nil {
		return
	}
	action := acts[i]
	key, rowAction := p.chosenRowKey()
	p.closeActionMenu()
	if rowAction {
		p.runRowAction(key, action)
		return
	}
	p.runAction(action)
}

// chosenRowKey is the learn key of the chosen row, and whether the actions the menu and the shortcuts run belong to a
// row: they belong to the page instead where no row is actionable.
func (p *Palette) chosenRowKey() (key string, isRow bool) {
	if it := p.actionableItem(); it != nil {
		return it.LearnKey, true
	}
	return "", false
}
