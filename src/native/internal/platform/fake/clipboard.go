package fake

import (
	"bytes"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Operation names accepted by Clipboard.FailNext.
const (
	OpClipboardReadText   = "ReadText"
	OpClipboardWriteText  = "WriteText"
	OpClipboardReadImage  = "ReadImagePNG"
	OpClipboardWriteImage = "WriteImagePNG"
	OpClipboardFormats    = "Formats"
	OpClipboardClear      = "Clear"
)

// Clipboard is an in-memory platform.Clipboard holding one text or one image, as the OS does: writing one
// replaces the other. Every change sends one value on Changed without blocking; a change that finds the
// channel full is coalesced with the pending one, as the port allows.
//
//	clip := fake.NewClipboard()
//	clip.WriteConcealedText("hunter2") // a password manager's copy
//	formats, _ := clip.Formats()       // includes platform.FormatConcealed
type Clipboard struct {
	mu        sync.Mutex
	text      string
	png       []byte
	concealed bool
	changed   chan struct{}
	faults    faultSet
}

var _ platform.Clipboard = (*Clipboard)(nil)

// NewClipboard returns an empty clipboard.
func NewClipboard() *Clipboard {
	return &Clipboard{changed: make(chan struct{}, 1)}
}

// FailNext makes the next call of op return err. Use the Op constants of this file.
func (c *Clipboard) FailNext(op string, err error) {
	c.faults.failNext(op, err)
}

// WriteConcealedText puts text on the clipboard marked as concealed, as a password manager does. Clipboard
// history must not record it.
func (c *Clipboard) WriteConcealedText(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setTextLocked(text, true)
}

// ReadText returns the plain text, or "" when the clipboard holds an image or nothing.
func (c *Clipboard) ReadText() (string, error) {
	if err := c.faults.take(OpClipboardReadText); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text, nil
}

// WriteText replaces the clipboard with plain text.
func (c *Clipboard) WriteText(text string) error {
	if err := c.faults.take(OpClipboardWriteText); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setTextLocked(text, false)
	return nil
}

// ReadImagePNG returns a copy of the image, or ErrNotFound when there is none.
func (c *Clipboard) ReadImagePNG() ([]byte, error) {
	if err := c.faults.take(OpClipboardReadImage); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.png == nil {
		return nil, platform.ErrNotFound
	}
	return bytes.Clone(c.png), nil
}

// WriteImagePNG replaces the clipboard with an image.
func (c *Clipboard) WriteImagePNG(png []byte) error {
	if err := c.faults.take(OpClipboardWriteImage); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.text = ""
	c.concealed = false
	c.png = bytes.Clone(png)
	c.notifyLocked()
	return nil
}

// Formats lists the content kinds on the clipboard. Empty when the clipboard holds nothing.
func (c *Clipboard) Formats() ([]platform.ClipboardFormat, error) {
	if err := c.faults.take(OpClipboardFormats); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var formats []platform.ClipboardFormat
	if c.text != "" {
		formats = append(formats, platform.FormatText)
	}
	if c.png != nil {
		formats = append(formats, platform.FormatPNG)
	}
	if c.concealed {
		formats = append(formats, platform.FormatConcealed)
	}
	return formats, nil
}

// Clear empties the clipboard.
func (c *Clipboard) Clear() error {
	if err := c.faults.take(OpClipboardClear); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.text, c.png, c.concealed = "", nil, false
	c.notifyLocked()
	return nil
}

// Changed returns the notification channel. It is never nil in the fake; capability clipboard.changed is
// reported as supported by NewSet.
func (c *Clipboard) Changed() <-chan struct{} {
	return c.changed
}

// setTextLocked stores text and notifies. The caller holds c.mu.
func (c *Clipboard) setTextLocked(text string, concealed bool) {
	c.text = text
	c.concealed = concealed
	c.png = nil
	c.notifyLocked()
}

// notifyLocked sends one coalesced change notification. The caller holds c.mu.
func (c *Clipboard) notifyLocked() {
	select {
	case c.changed <- struct{}{}:
	default:
	}
}
