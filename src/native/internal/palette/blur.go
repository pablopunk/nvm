package palette

import "time"

// blurDismissGrace is how long a focus loss waits before it hides the palette. The summon shortcut's own grab
// takes the keyboard for a moment, and the window reports that as a focus loss, so the shortcut's press must
// still find the palette shown in order to hide it.
const blurDismissGrace = 200 * time.Millisecond

// DismissOnBlur hides the palette after blurDismissGrace, unless the palette was hidden or shown again in the
// meantime. The host calls it when the window loses focus. It is safe from the window's thread.
func (p *Palette) DismissOnBlur() {
	shows := p.shows
	go func() {
		<-p.opts.Clock.After(blurDismissGrace)
		p.win.Update(func() {
			if p.shows == shows && p.win.IsVisible() {
				p.hideAfterFocusMovedAway()
			}
		})
	}()
}
