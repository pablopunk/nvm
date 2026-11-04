package commands

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Replace puts text in place of the selection in source. The palette is dismissed first. The text goes on the
// clipboard before anything else, so a failed paste still leaves it there. The paste runs off the UI thread, then the
// clipboard the replace displaced comes back on the UI thread once RestoreDelay has passed, as long as it still holds
// the text.
func (s *Service) Replace(text string, source platform.FocusTarget) {
	s.opts.Surface.DismissPalette()
	previous, _ := s.opts.Clipboard.ReadText()
	s.opts.Background("ai-commands.replace", func() {
		err := s.paste(text, source)
		s.opts.Dispatch(func() { s.replaceFinished(text, previous, err) })
	})
}

// paste places text on the clipboard, gives the keyboard back to source and sends the paste gesture.
func (s *Service) paste(text string, source platform.FocusTarget) error {
	if err := writeConcealed(s.opts.Clipboard, text); err != nil {
		return err
	}
	if err := restoreSource(s.opts.Focus, source); err != nil {
		return err
	}
	return s.opts.Paster.PasteIntoFrontmost("")
}

// replaceFinished runs on the UI thread when a replace ends. A failed paste is said on the palette and the text stays
// on the clipboard; a paste that landed schedules the restore of the clipboard it displaced.
func (s *Service) replaceFinished(text, previous string, err error) {
	if err != nil {
		s.opts.Surface.Toast(pasteFailureMessage(err))
		return
	}
	s.opts.After(s.opts.RestoreDelay, func() { s.restoreAfterReplace(text, previous) })
}

// restoreAfterReplace puts the previous clipboard back, unless the user has copied something since.
func (s *Service) restoreAfterReplace(text, previous string) {
	if current, err := s.opts.Clipboard.ReadText(); err == nil && current == text {
		_ = restorePrevious(s.opts.Clipboard, previous)
	}
}

// restorePrevious puts the clipboard text from before a replace back, concealed.
func restorePrevious(clip platform.Clipboard, previous string) error {
	if previous == "" {
		return clip.Clear()
	}
	return writeConcealed(clip, previous)
}

// pasteFailureMessage says why the replacement could not be pasted, and that the text is still on the clipboard.
func pasteFailureMessage(err error) string {
	switch {
	case errors.Is(err, platform.ErrUnsupported):
		return "Paste is not available on this system yet. The text is on the clipboard."
	case errors.Is(err, platform.ErrConflict):
		return "The focused app changed, so nothing was pasted."
	case errors.Is(err, platform.ErrDenied):
		return "Allow Nevermind to control the keyboard in System Settings to paste."
	case errors.Is(err, errSourceGone):
		return "The source window is gone. The text is on the clipboard."
	}
	return "Paste failed. The text is on the clipboard."
}
