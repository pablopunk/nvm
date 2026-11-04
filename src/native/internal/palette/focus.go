package palette

// captureFocus remembers the window that had the keyboard before the palette takes it. Nothing is remembered when
// the host has no focus port or no window has the keyboard.
func (p *Palette) captureFocus() {
	p.returnTo = nil
	if p.opts.Focus == nil {
		return
	}
	if target, err := p.opts.Focus.Active(); err == nil {
		p.returnTo = &target
	}
}

// returnFocus gives the keyboard back to the window captured on show. A window that has gone has nowhere to
// return to, so a failed restore is not shown to the user.
func (p *Palette) returnFocus() {
	target := p.returnTo
	p.returnTo = nil
	if target != nil && p.opts.Focus != nil {
		_ = p.opts.Focus.Restore(*target)
	}
}

// hideAfterAction dismisses the palette for an action that chose where the keyboard goes: the app it opens, or
// the one it pastes into. The palette does not send the keyboard back over that choice.
func (p *Palette) hideAfterAction() {
	p.returnTo = nil
	p.Hide()
}

// hideAfterFocusMovedAway dismisses the palette after another window took the keyboard, as a click on it does. The
// keyboard stays with the window the user chose instead of going back to the one from before the summon.
func (p *Palette) hideAfterFocusMovedAway() {
	p.returnTo = nil
	p.Hide()
}
