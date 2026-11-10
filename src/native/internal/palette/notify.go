package palette

import "github.com/pablopunk/nvm/native/internal/palette/feedback"

// Feedback is a message for the user: the title of the action that raised it (empty for the app's name), its text
// and how it reads.
type Feedback struct {
	Title, Message string
	Tone           feedback.Tone
}

// Toast sends a message for the user to the HUD. A toast raised while an action runs carries that action's title, as
// 1.x's action feedback does.
func (p *Palette) Toast(msg string) {
	p.Notify(Feedback{Title: p.running, Message: msg})
}

// Notify sends f to the HUD, whether the palette is up or hidden: every action result reads there, as in 1.x.
func (p *Palette) Notify(f Feedback) {
	if p.opts.Feedback != nil {
		p.opts.Feedback(f)
	}
}
