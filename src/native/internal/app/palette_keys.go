package app

import (
	"github.com/pablopunk/nvm/native/internal/search"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Keyboard routing of the palette: the keys each level handles before the input, the action menu, and Enter.

func (p *Palette) keys(c *ui.Context, v *View) {
	if p.actionsOpen {
		acts := p.actions()
		switch {
		case c.Shortcut(0, ui.KeyEscape), c.Shortcut(mod, ui.KeyK):
			p.actionsOpen = false
		case c.Shortcut(0, ui.KeyDown):
			p.actChosen = min(p.actChosen+1, len(acts)-1)
		case c.Shortcut(0, ui.KeyUp):
			p.actChosen = max(p.actChosen-1, 0)
		}
		return
	}
	if v.OnKey != nil && v.OnKey(c, p) {
		return
	}
	if c.Shortcut(mod, ui.KeyK) && len(p.actions()) > 0 {
		p.actionsOpen, p.actChosen = true, 0
		return
	}
	if len(p.stack) == 1 && strings.TrimSpace(p.query) != "" && c.Shortcut(0, ui.KeyTab) {
		p.Push(p.newChatView(strings.TrimSpace(p.query)))
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		switch {
		case p.query != "":
			p.query = ""
			p.search()
		case len(p.stack) > 1:
			p.Pop()
		default:
			p.Hide()
		}
		return
	}
	if p.query == "" && len(p.stack) > 1 && !v.HideInput && v.Body == nil && c.Shortcut(0, ui.KeyBackspace) {
		p.Pop()
		return
	}
	if v.Body != nil {
		return
	}
	if c.Shortcut(0, ui.KeyDown) {
		p.choose(p.chosen + 1)
	}
	if c.Shortcut(0, ui.KeyUp) {
		p.choose(p.chosen - 1)
	}
	if it := p.chosenItem(); it != nil {
		for _, a := range it.Actions[min(1, len(it.Actions)):] {
			if a.Key != 0 && c.Shortcut(a.Mods, a.Key) {
				a.Run(p)
				return
			}
		}
	}
}

// onEnter is Enter in the input: run the chosen action, or submit the text in input-driven views.
func (p *Palette) onEnter() {
	v := p.cur()
	switch {
	case p.actionsOpen:
		acts := p.actions()
		p.actionsOpen = false
		if p.actChosen < len(acts) {
			acts[p.actChosen].Run(p)
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

func (p *Palette) actions() []search.Action {
	if it := p.chosenItem(); it != nil {
		return it.Actions
	}
	return nil
}
