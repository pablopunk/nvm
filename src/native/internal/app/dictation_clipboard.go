package app

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// writeDictation puts text on the clipboard: concealed unless the user asked to keep it there.
func writeDictation(clipboard platform.Clipboard, text string, keep bool) error {
	if keep {
		return clipboard.WriteText(text)
	}
	concealed, ok := clipboard.(platform.ConcealedClipboard)
	if !ok {
		return platform.ErrUnsupported
	}
	return concealed.WriteConcealedText(text)
}

// dictationClipboardMessage says why the text could not be placed on the clipboard.
func dictationClipboardMessage(err error) string {
	if err == platform.ErrUnsupported {
		return "Dictation needs a clipboard that keeps text out of history, which this system does not offer yet."
	}
	return "Dictation could not reach the clipboard."
}

// dictationPasteMessage says why the text was not pasted; the text stays on the clipboard in every case.
func dictationPasteMessage(err error) string {
	switch {
	case errors.Is(err, platform.ErrConflict):
		return "The app you dictated into is not in front, so nothing was pasted. The text is on the clipboard."
	case errors.Is(err, platform.ErrDenied):
		return "Allow Nevermind to control the keyboard in System Settings to paste. The text is on the clipboard."
	case errors.Is(err, platform.ErrUnsupported):
		return "Paste is not available on this system yet. The text is on the clipboard."
	}
	return "Paste failed. The text is on the clipboard."
}

// restoreDictationClipboard puts the previous text back only when the clipboard still holds the dictation, so a copy
// the user made meanwhile is kept.
func restoreDictationClipboard(clipboard platform.Clipboard, dictated, previous string) {
	if current, err := clipboard.ReadText(); err == nil && current == dictated {
		_ = clipboard.WriteText(previous)
	}
}
