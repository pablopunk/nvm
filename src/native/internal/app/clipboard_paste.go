package app

import (
	"context"
	"errors"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// clipPasteName names the paste goroutine for panic reporting.
const clipPasteName = "clipboard.paste"

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

// pasteAfterHide hides the palette and pastes e into the app that had focus. Stale hide signals are dropped
// first, so the paste waits for this hide. The paste runs on its own goroutine, because it waits for the hide the
// UI thread is in the middle of.
func (p *App) pasteAfterHide(h search.Host, e clipboard.Entry) {
	select {
	case <-p.paletteHidden:
	default:
	}
	h.Hide()
	clip := p.clip
	safego.Go(clipPasteName, func(context.Context) {
		if err := clip.paste(e); err != nil {
			p.toastFromBackground(pasteFailureMessage(err))
		}
	})
}

// pasteFailureMessage says what went wrong in a paste, in words the user can act on.
func pasteFailureMessage(err error) string {
	switch {
	case errors.Is(err, platform.ErrUnsupported):
		return "Paste is not available on this system yet. The item is on the clipboard; press Ctrl+V to paste it."
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

// redraw runs fn on the palette's UI thread and redraws. Before the window exists there is nothing to draw, so
// the call does nothing.
func (p *App) redraw(fn func()) {
	if p.panel == nil {
		return
	}
	p.panel.Update(fn)
}

// toastFromBackground shows a one-line message from a background goroutine.
func (p *App) toastFromBackground(msg string) {
	p.redraw(func() { p.Toast(msg) })
}
