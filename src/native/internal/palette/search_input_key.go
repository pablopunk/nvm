package palette

import (
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// onSearchInputKey routes empty-query Backspace before the focused text input consumes it. Other search keys retain
// their existing edge and Enter handling.
func (p *Palette) onSearchInputKey(k ui.TextKey) bool {
	if k.Key == ui.KeyEnter {
		diag.ActionTrace("input-enter", logx.F{"menu_open": p.actionsOpen})
	}
	v := p.cur()
	if k.Mods == 0 && k.Key == ui.KeyBackspace && k.Text() == "" && !p.actionsOpen &&
		len(p.stack) > 1 && !v.HideInput && v.Body == nil {
		p.Pop()
		return true
	}
	return p.onSearchKey(k)
}
