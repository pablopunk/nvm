package palette

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// keys routes the keyboard for the view on top, in the order the contract fixes: the action menu first, then the
// view's own keys, then the menu's toggle and Tab, then Escape, then Backspace, then the list, and the shortcuts of the
// actions. Command-K never strands focus: it toggles the menu, and the menu closes on Escape or Command-K.
func (p *Palette) keys(c *ui.Context, v *View) {
	if side := v.sideNow(); side.Focused() {
		side.keys(c)
		return
	}
	if edge := p.takeSearchEdge(); edge != edgeNone {
		p.runSearchEdge(edge)
		return
	}
	if p.actionsOpen {
		switch {
		case c.Shortcut(0, ui.KeyEscape), c.Shortcut(Mod, ui.KeyK):
			p.closeActionMenu()
		case c.Shortcut(0, ui.KeyDown):
			p.moveMenuChoice(1)
		case c.Shortcut(0, ui.KeyUp):
			p.moveMenuChoice(-1)
		case c.Shortcut(0, ui.KeyTab):
		default:
			p.runActionShortcut(c)
		}
		return
	}
	if v.OnKey != nil && v.OnKey(c, p) {
		return
	}
	if c.Shortcut(Mod, ui.KeyK) && len(p.menuActions()) > 0 {
		p.openActionMenu(0)
		return
	}
	if len(p.stack) == 1 && p.opts.AskAI != nil && strings.TrimSpace(p.query) != "" && c.Shortcut(0, ui.KeyTab) {
		p.rememberQuery(p.query)
		p.push(p.opts.AskAI(strings.TrimSpace(p.query)))
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		p.escape(v)
		return
	}
	if p.query == "" && (len(p.stack) > 1 || p.entryBoundary > 0) && !v.HideInput && v.Body == nil && c.Shortcut(0, ui.KeyBackspace) {
		p.Pop()
		return
	}
	if v.Body == nil && p.navigateRows(c, v) {
		return
	}
	p.runActionShortcut(c)
}

// navigateRows moves the cursor of a list or grid by the arrows. It reports whether the root's query history took the
// keys, which ends the keys of the frame.
func (p *Palette) navigateRows(c *ui.Context, v *View) bool {
	if v.Grid != nil {
		p.moveGridChoice(c, v.Grid)
		return false
	}
	down, up := c.Shortcut(0, ui.KeyDown), c.Shortcut(0, ui.KeyUp)
	if p.stepRootHistory(up, down) {
		return true
	}
	if down {
		p.choose(p.chosen + 1)
	}
	if up {
		p.choose(p.chosen - 1)
	}
	return false
}

// runActionShortcut runs the first action whose key was pressed: the chosen row's, then the page's own, as a row's
// shortcut does.
func (p *Palette) runActionShortcut(c *ui.Context) {
	for _, a := range p.shortcutActions() {
		if a.Key != 0 && c.Shortcut(a.Mods, a.Key) {
			if p.actionsOpen {
				p.closeActionMenu()
			}
			p.runAction(a)
			return
		}
	}
}

// escape is the palette's Escape, as 1.x reads it: the root's history walk ends, then the page may use the key, then a
// pushed page is left, and at the root the palette hides, whatever the query holds.
func (p *Palette) escape(v *View) {
	switch {
	case p.restoreRootDraft():
	case v.OnEscape != nil && v.OnEscape():
	case len(p.stack) > 1:
		if p.entryBoundary > 0 && len(p.stack) == p.entryBoundary+1 {
			p.Hide()
		} else {
			p.Pop()
		}
	default:
		p.Hide()
	}
}
