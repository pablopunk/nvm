package selection

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Reading the selection polls the clipboard for the copy's result: pollAttempts reads, pollInterval apart.
const (
	pollAttempts = 20
	pollInterval = 25 * time.Millisecond
)

// Reader copies the selection of the source app through the copy gesture and restores the clipboard. Before the
// gesture it puts a random marker on the clipboard, concealed so clipboard history never records it; a copy has
// landed when the clipboard holds anything else.
type Reader struct {
	clock     platform.Clock
	clip      platform.Clipboard
	selection platform.SelectedText
	focus     platform.Focus
	marker    func() string
}

// NewReader returns a reader over the ports. focus may be nil where the OS has no focus port.
func NewReader(clock platform.Clock, clip platform.Clipboard, selection platform.SelectedText, focus platform.Focus) *Reader {
	return &Reader{clock: clock, clip: clip, selection: selection, focus: focus, marker: newMarker}
}

// Read returns the text selected in source, or "" when nothing was selected. The clipboard is put back as it was.
// It returns ErrSourceGone when the source window has gone and ErrFormattedClipboard when the clipboard holds
// content the restore could not preserve.
func (r *Reader) Read(source platform.FocusTarget) (string, error) {
	if err := restoreSource(r.clock, r.focus, source); err != nil {
		return "", err
	}
	snapshot, err := r.snapshot()
	if err != nil {
		return "", err
	}
	defer r.restore(snapshot)
	marker := r.marker()
	if err := writeConcealed(r.clip, marker); err != nil {
		return "", err
	}
	if err := r.selection.CopySelection(""); err != nil {
		return "", err
	}
	return r.awaitCopy(marker)
}

// keyboardWait bounds the wait for the OS to report the source window as active after the keyboard is given back.
const keyboardWait = time.Second

// restoreSource gives the keyboard back to the source window, then waits until the OS reports it there, so a copy or
// paste gesture reaches the source rather than a palette that is still up. A window that has gone is ErrSourceGone;
// one that never takes the keyboard is ErrKeyboardHeld. An OS without the focus port keeps the keyboard where the
// palette left it.
func restoreSource(clock platform.Clock, focus platform.Focus, source platform.FocusTarget) error {
	if source.Window == "" || focus == nil {
		return nil
	}
	if err := focus.Restore(source); errors.Is(err, platform.ErrNotFound) {
		return ErrSourceGone
	}
	return awaitKeyboard(clock, focus, source)
}

// awaitKeyboard polls the focus port until source has the keyboard, for at most keyboardWait.
func awaitKeyboard(clock platform.Clock, focus platform.Focus, source platform.FocusTarget) error {
	deadline := clock.Now().Add(keyboardWait)
	for {
		if active, err := focus.Active(); err == nil && active.Window == source.Window {
			return nil
		}
		if !clock.Now().Before(deadline) {
			return ErrKeyboardHeld
		}
		<-clock.After(pollInterval)
	}
}

// awaitCopy polls until the clipboard holds something other than marker. The copied text is concealed at once, as
// the desktop app does, so the watcher skips it. An empty result means nothing was selected.
func (r *Reader) awaitCopy(marker string) (string, error) {
	for attempt := 1; attempt <= pollAttempts; attempt++ {
		text, err := r.clip.ReadText()
		if err != nil {
			return "", err
		}
		if text != marker {
			if text == "" {
				return "", nil
			}
			return text, writeConcealed(r.clip, text)
		}
		if attempt < pollAttempts {
			<-r.clock.After(pollInterval)
		}
	}
	return "", nil
}

// clipboardSnapshot is the plain text that was on the clipboard before the read.
type clipboardSnapshot struct {
	text string
}

// snapshot records the clipboard's plain text. Formatted content (HTML, RTF, files, images) cannot be put back
// exactly, so the read refuses it rather than losing it.
func (r *Reader) snapshot() (clipboardSnapshot, error) {
	formats, err := r.clip.Formats()
	if err != nil && !errors.Is(err, platform.ErrUnsupported) {
		return clipboardSnapshot{}, err
	}
	for _, format := range formats {
		if isFormatted(format) {
			return clipboardSnapshot{}, ErrFormattedClipboard
		}
	}
	text, err := r.clip.ReadText()
	if err != nil {
		return clipboardSnapshot{}, err
	}
	return clipboardSnapshot{text: text}, nil
}

// restore puts the snapshot back, concealed so the watcher does not record the restored text as a new copy.
func (r *Reader) restore(snapshot clipboardSnapshot) {
	if snapshot.text == "" {
		_ = r.clip.Clear()
		return
	}
	_ = writeConcealed(r.clip, snapshot.text)
}

// isFormatted reports whether a clipboard format holds content that a plain-text restore would lose.
func isFormatted(format platform.ClipboardFormat) bool {
	switch format {
	case platform.FormatHTML, platform.FormatRTF, platform.FormatFiles, platform.FormatPNG:
		return true
	}
	return false
}

// writeConcealed puts text on the clipboard with the concealed marker when the clipboard supports it, and as plain
// text otherwise.
func writeConcealed(clip platform.Clipboard, text string) error {
	if concealed, ok := clip.(platform.ConcealedClipboard); ok {
		return concealed.WriteConcealedText(text)
	}
	return clip.WriteText(text)
}

// newMarker returns a random marker that no copy of the user's text can equal.
func newMarker() string {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	return "__NEVERMIND_SELECTION_" + hex.EncodeToString(raw[:]) + "__"
}
