package palette

import "github.com/egoist/mygo/ui"

// focusListIfRequested takes the keyboard to the list the frame has just built, when the view hides its field.
func (p *Palette) focusListIfRequested(c *ui.Context) {
	if p.focusList && p.list.Focus(c) {
		p.focusList = false
	}
}
