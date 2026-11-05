package app

import (
	"context"
	"errors"
	"runtime"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/palette/feedback"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
	"github.com/pablopunk/nvm/native/internal/shortcuts"
)

// clipPasteName names the paste goroutine for panic reporting.
const clipPasteName = "clipboard.paste"

// appPorts returns the production ports, with the paste gesture waiting for the palette to hide. ready builds the
// app's one port set from it, so snippets, emoji and the clipboard all paste the same way.
func (p *App) appPorts() platform.Ports {
	ports := real.NewPorts()
	ports.Paster = real.NewPaster(ports.Helper, p.waitPaletteHidden)
	return rigPorts(ports)
}

// waitPaletteHidden checks visibility on the UI thread after the caller's dismissal and focus restoration. A hidden
// palette stays ready for repeated background pastes; a newer summon refuses the gesture instead of consuming it.
func (p *App) waitPaletteHidden(ctx context.Context) error {
	ready := make(chan error, 1)
	p.onUI(func() {
		if err := ctx.Err(); err != nil {
			ready <- err
		} else if p.Palette != nil && p.Visible() {
			ready <- platform.ErrConflict
		} else {
			ready <- nil
		}
	})
	select {
	case err := <-ready:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// pasteActionTitle is the title of the paste action that copies, emoji and snippets offer. The HUD names a paste by it.
const pasteActionTitle = "Paste"

// hideThenRun hides the palette, then runs run on its own goroutine once the palette has hidden, so the keys of a paste
// reach the app that had focus. The paste port checks visibility after this dismissal. A failure shows the
// feedback fail builds; success shows done under title.
func (p *App) hideThenRun(h search.Host, title, done string, run func() error, fail func(error) palette.Feedback) {
	h.Hide()
	safego.Go(clipPasteName, func(context.Context) {
		if err := run(); err != nil {
			p.feedbackFromBackground(fail(err))
			return
		}
		p.redraw(func() { p.showFeedback(title, done, feedback.ToneDefault) })
	})
}

// pasteFailedFeedback is the error the HUD shows for a paste that failed, under the title of the action that tried it.
func pasteFailedFeedback(title string, err error) palette.Feedback {
	return palette.Feedback{Title: title, Message: pasteFailureMessage(err), Tone: feedback.ToneError}
}

// pasteFailureMessage says what went wrong in a paste, in words the user can act on.
func pasteFailureMessage(err error) string {
	switch {
	case errors.Is(err, platform.ErrUnsupported):
		return "Paste is not available on this system yet. The item is on the clipboard; press " + shortcuts.Label("Control+V", runtime.GOOS) + " to paste it."
	case errors.Is(err, platform.ErrConflict):
		return "The focused app changed, so nothing was pasted."
	case errors.Is(err, platform.ErrDenied):
		return "Allow Nevermind to control the keyboard in System Settings to paste."
	}
	return "Paste failed. The item is on the clipboard."
}

// copyFailureMessage says what went wrong in a copy.
func copyFailureMessage(err error) string {
	if errors.Is(err, platform.ErrUnsupported) {
		return "Copying this item is not available on this system yet."
	}
	return "Copy failed."
}

// copyText puts plain text on the clipboard for a Copy action. A failure is shown as a toast. A copy the user makes
// is recorded in the history like any other copy.
func (p *App) copyText(h search.Host, text string) {
	if err := p.ports.Clipboard.WriteText(text); err != nil {
		h.Toast("Copy failed: " + err.Error())
	}
}

// redraw runs fn on the palette's UI thread and redraws. Before the window exists there is nothing to draw, so
// the call does nothing.
func (p *App) redraw(fn func()) {
	if p.panel == nil {
		return
	}
	p.panel.Update(fn)
}

// feedbackFromBackground shows f from a background goroutine, under the field or in the HUD, as the palette is up.
func (p *App) feedbackFromBackground(f palette.Feedback) {
	p.redraw(func() { p.Notify(f) })
}
