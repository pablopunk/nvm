package selection

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Reading the selection polls the clipboard for the copy's result: pollAttempts reads, pollInterval apart.
const (
	pollAttempts = 40
	pollInterval = 25 * time.Millisecond
)

// Reader copies the selection of the source app through the copy gesture and restores the clipboard. Before the
// gesture it puts a random marker on the clipboard, concealed so clipboard history never records it; a copy has
// landed when the clipboard holds anything else.
type Reader struct {
	pending   *clipboardSnapshot
	mu        sync.Mutex
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

// Read returns the text selected in source, or "" when nothing was selected. The OS accessibility API answers first,
// without touching the clipboard; an app that does not expose its selection (Electron apps such as Slack often do
// not) is read through the copy gesture, and the clipboard is put back as it was. A clipboard without full snapshot
// support refuses rich content it could not preserve.
func (r *Reader) Read(source platform.FocusTarget) (text string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := restoreSource(r.clock, r.focus, source); err != nil {
		return "", err
	}
	if text := r.readAccessible(); text != "" {
		return text, nil
	}
	snapshot, err := r.snapshot()
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, r.restore(snapshot)) }()
	marker := r.marker()
	if err := r.writeTemporary(&snapshot, marker); err != nil {
		return "", err
	}
	if err := r.selection.CopySelection(""); err != nil {
		return "", err
	}
	return r.awaitCopy(marker, &snapshot)
}

// readAccessible asks the accessibility API for the selection. Any failure is an empty answer: the copy gesture runs
// next and reports what is still wrong, such as a missing permission.
func (r *Reader) readAccessible() string {
	accessible, ok := r.selection.(platform.AccessibleSelection)
	if !ok {
		return ""
	}
	text, err := accessible.ReadAccessibleSelection()
	if err != nil {
		return ""
	}
	return text
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
func (r *Reader) awaitCopy(marker string, snapshot *clipboardSnapshot) (string, error) {
	for attempt := 1; attempt <= pollAttempts; attempt++ {
		text, err := r.clip.ReadText()
		if err != nil {
			return "", err
		}
		if text != marker {
			if text == "" {
				return "", nil
			}
			return text, r.writeTemporary(snapshot, text)
		}
		if attempt < pollAttempts {
			<-r.clock.After(pollInterval)
		}
	}
	return "", nil
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
