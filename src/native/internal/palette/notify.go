package palette

import "github.com/pablopunk/nvm/native/internal/palette/feedback"

// Feedback is a message for the user: the title of the action that raised it (empty for the app's name), its text
// and how it reads.
type Feedback struct {
	Title, Message string
	Tone           feedback.Tone
}

// Toast sets the one-line message shown under the search field on the next frame. A toast raised while an action runs
// carries that action's title, as 1.x's action feedback does.
func (p *Palette) Toast(msg string) {
	p.Notify(Feedback{Title: p.running, Message: msg})
}

// Notify shows f under the search field when the palette is up. A message raised while the palette is hidden is not
// drawn under the field, which belongs to the presentation that is open: it goes to Feedback.
func (p *Palette) Notify(f Feedback) {
	if p.win == nil {
		return
	}
	if p.win.IsVisible() {
		p.toastMsg = f.Message
		return
	}
	if p.opts.Feedback != nil {
		p.opts.Feedback(f)
	}
}
