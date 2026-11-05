package palette

import "github.com/egoist/mygo/ui"

// onSearchInputKey routes empty-query Backspace before the focused text input consumes it. Other search keys retain
// their existing edge and Enter handling.
func (p *Palette) onSearchInputKey(k ui.TextKey) bool {
	v := p.cur()
	if k.Mods == 0 && k.Key == ui.KeyBackspace && k.Text() == "" && !p.actionsOpen &&
		len(p.stack) > 1 && !v.HideInput && v.Body == nil {
		p.Pop()
		return true
	}
	return p.onSearchKey(k)
}
