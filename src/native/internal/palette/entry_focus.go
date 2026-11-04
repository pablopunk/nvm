package palette

import "github.com/egoist/mygo/ui"

// focusSearch gives the keyboard to the search field of the view on top. A view that hides its field gives it to its
// list instead, once the list is built, so Return and the arrows still reach the rows.
func (p *Palette) focusSearch() {
	p.input.Focus()
	v := p.cur()
	p.focusList = v.HideInput && v.Body == nil
}

// focusListIfRequested takes the keyboard to the list the frame has just built, when the view hides its field.
func (p *Palette) focusListIfRequested(c *ui.Context) {
	if p.focusList && p.list.Focus(c) {
		p.focusList = false
	}
}
