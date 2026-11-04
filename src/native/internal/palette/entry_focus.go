package palette

import "github.com/egoist/mygo/ui"

// focusInput gives the keyboard to the text input of the page on top: the input the page draws itself when it owns
// one, otherwise the search field. A view that hides its field hands the keyboard to its list once the list is built.
func (p *Palette) focusInput() {
	p.cur().sideNow().unfocus()
	if owned := p.cur().Input; owned != nil {
		owned.Focus()
		p.focusList = false
		return
	}
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
