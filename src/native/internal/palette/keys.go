package palette

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// keys routes the keyboard for the view on top, in the order the contract fixes: the action menu first, then the
// view's own keys, then the menu's toggle and Tab, then Escape, then Backspace, then the list and the rows'
// shortcuts. Command-K never strands focus: it toggles the menu, and the menu closes on Escape or Command-K.
func (p *Palette) keys(c *ui.Context, v *View) {
	if p.actionsOpen {
		acts := p.actions()
		switch {
		case c.Shortcut(0, ui.KeyEscape), c.Shortcut(Mod, ui.KeyK):
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
	if c.Shortcut(Mod, ui.KeyK) && len(p.actions()) > 0 {
		p.actionsOpen, p.actChosen = true, 0
		return
	}
	if len(p.stack) == 1 && p.opts.AskAI != nil && strings.TrimSpace(p.query) != "" && c.Shortcut(0, ui.KeyTab) {
		p.rememberQuery(p.query)
		p.push(p.opts.AskAI(strings.TrimSpace(p.query)))
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		switch {
		case p.restoreRootDraft():
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
	if v.Grid != nil {
		p.moveGridChoice(c, v.Grid)
	} else {
		down, up := c.Shortcut(0, ui.KeyDown), c.Shortcut(0, ui.KeyUp)
		if p.stepRootHistory(up, down) {
			return
		}
		if down {
			p.choose(p.chosen + 1)
		}
		if up {
			p.choose(p.chosen - 1)
		}
	}
	if it := p.chosenItem(); it != nil {
		for _, a := range it.Actions {
			if a.Key != 0 && c.Shortcut(a.Mods, a.Key) {
				p.runAction(a)
				return
			}
		}
	}
}
