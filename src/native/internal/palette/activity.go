package palette

// syncActive tells the top view whether the palette shows it as the active page, and the covered view, when there is
// one, that it no longer is. Both calls are idempotent, so a view may hear the same state twice.
func (p *Palette) syncActive(covered *View) {
	if covered != nil && covered != p.cur() {
		notifyActive(covered, false)
	}
	notifyActive(p.cur(), p.win != nil && p.win.IsVisible())
}

func notifyActive(v *View, active bool) {
	if v.OnActive != nil {
		v.OnActive(active)
	}
}
