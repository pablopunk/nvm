package palette

type windowOpacity interface{ SetOpacity(float64) }

// showPreparedWindow keeps the previous backing frame out of sight until the new view has rendered.
func (p *Palette) showPreparedWindow() {
	if opacity, ok := p.win.(windowOpacity); ok && (!p.win.IsVisible() || p.revealWaiting) {
		opacity.SetOpacity(0)
		p.revealWaiting = true
		p.revealFrame = p.shows
	}
	p.win.Show()
}

func (p *Palette) revealRenderedWindow() {
	if p.revealFrame == 0 {
		return
	}
	shows := p.revealFrame
	p.revealFrame = 0
	p.win.Update(func() {
		if shows == p.shows && p.win.IsVisible() {
			p.win.(windowOpacity).SetOpacity(1)
			p.revealWaiting = false
			if p.opts.OnPresented != nil {
				p.opts.OnPresented()
			}
		}
	})
}
