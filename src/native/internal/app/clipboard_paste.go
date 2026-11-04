package app

import (
	"context"
	"errors"
	"runtime"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
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

// notePaletteHidden records that the palette window hid. The window calls it from its hide event on the UI
// thread; the buffered signal is taken by the next paste, so the event is never lost and never waited for on the
// UI thread.
func (p *App) notePaletteHidden() {
	select {
	case p.paletteHidden <- struct{}{}:
	default:
	}
}

// waitPaletteHidden returns once the palette has hidden since the last paste started. The paste port calls it on
// its own goroutine, before it sends the gesture, so keys never reach the palette.
func (p *App) waitPaletteHidden(ctx context.Context) error {
	select {
	case <-p.paletteHidden:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// pasteActionTitle is the title of the paste action that copies, emoji and snippets offer. The HUD names a paste by it.
const pasteActionTitle = "Paste"

// pasteAfterHide hides the palette and pastes e into the app that had focus, for the action titled title.
func (p *App) pasteAfterHide(h search.Host, title string, e clipboard.Entry) {
	clip := p.clip
	p.hideThenRun(h, title, "Pasted", func() error { return clip.paste(e) }, func(err error) palette.Feedback {
		return pasteFailedFeedback(title, err)
	})
}

// hideThenRun hides the palette, then runs run on its own goroutine once the palette has hidden, so the keys of a paste
// reach the app that had focus. Stale hide signals are dropped first, so run waits for this hide. A failure shows the
// feedback fail builds; success shows done under title.
func (p *App) hideThenRun(h search.Host, title, done string, run func() error, fail func(error) palette.Feedback) {
	select {
	case <-p.paletteHidden:
	default:
	}
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

// toastFromBackground shows a one-line message from a background goroutine.
func (p *App) toastFromBackground(msg string) {
	p.redraw(func() { p.Toast(msg) })
}

// canPaste reports whether the platform can send the paste gesture. The emoji grid copies where it cannot.
func (p *App) canPaste() bool {
	return p.ports.Capabilities.Supported(platform.CapabilityClipboardPaste)
}
