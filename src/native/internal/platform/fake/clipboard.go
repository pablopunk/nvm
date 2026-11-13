package fake

import (
	"bytes"
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Operation names accepted by Clipboard.FailNext.
const (
	OpClipboardReadText   = "ReadText"
	OpClipboardWriteText  = "WriteText"
	OpClipboardReadImage  = "ReadImagePNG"
	OpClipboardWriteImage = "WriteImagePNG"
	// OpClipboardWriteConcealed fails WriteConcealedText.
	OpClipboardWriteConcealed = "WriteConcealedText"
	OpClipboardFormats        = "Formats"
	OpClipboardClear          = "Clear"
)

// Clipboard is an in-memory platform.Clipboard holding one kind of content at a time, as the OS does:
// writing text, an image, or files replaces what was there. Every change sends one value on Changed
// without blocking; a change that finds the channel full is coalesced with the pending one, as the port
// allows. It also implements platform.RichClipboard for HTML, RTF and file lists.
//
//	clip := fake.NewClipboard()
//	_ = clip.WriteConcealedText("hunter2") // a password manager's copy
//	formats, _ := clip.Formats()       // includes platform.FormatConcealed
type Clipboard struct {
	mu        sync.Mutex
	text      string
	png       []byte
	html      string
	rtf       string
	files     []string
	concealed bool
	changed   chan struct{}
	revision  uint64
	faults    faultSet
}

var (
	_ platform.Clipboard     = (*Clipboard)(nil)
	_ platform.RichClipboard = (*Clipboard)(nil)
)

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
func (c *Clipboard) WriteConcealedText(text string) error {
	if err := c.faults.take(OpClipboardWriteConcealed); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setTextLocked(text, true)
	return nil
}

// WriteConcealedImagePNG puts an image on the clipboard marked as concealed.
func (c *Clipboard) WriteConcealedImagePNG(png []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearLocked()
	c.png = bytes.Clone(png)
	c.concealed = true
	c.notifyLocked()
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
	c.clearLocked()
	c.png = bytes.Clone(png)
	c.notifyLocked()
	return nil
}

// ReadHTML returns the HTML markup, or "" when the clipboard holds none.
func (c *Clipboard) ReadHTML() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.html, nil
}

// ReadRTF returns the RTF markup, or "" when the clipboard holds none.
func (c *Clipboard) ReadRTF() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rtf, nil
}

// ReadFilePaths returns a copy of the file paths on the clipboard, or nil.
func (c *Clipboard) ReadFilePaths() ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.files), nil
}

// WriteRich replaces the clipboard with plain text and HTML markup.
func (c *Clipboard) WriteRich(text, html string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setTextLocked(text, false)
	c.html = html
	return nil
}

// SetFilePaths replaces the clipboard with a list of files, as a file manager copy does.
func (c *Clipboard) SetFilePaths(paths ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearLocked()
	c.files = slices.Clone(paths)
	c.notifyLocked()
}

// WriteFiles replaces the clipboard with a list of files, as the real port does.
func (c *Clipboard) WriteFiles(paths []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearLocked()
	c.files = slices.Clone(paths)
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
	if c.html != "" {
		formats = append(formats, platform.FormatHTML)
	}
	if c.rtf != "" {
		formats = append(formats, platform.FormatRTF)
	}
	if c.png != nil {
		formats = append(formats, platform.FormatPNG)
	}
	if len(c.files) > 0 {
		formats = append(formats, platform.FormatFiles)
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
	c.clearLocked()
	c.notifyLocked()
	return nil
}

// Changed returns the notification channel. It is never nil in the fake; capability clipboard.changed is
// reported as supported by NewSet.
func (c *Clipboard) Changed() <-chan struct{} {
	return c.changed
}

// clearLocked empties every format. The caller holds c.mu and notifies afterwards.
func (c *Clipboard) clearLocked() {
	c.text, c.png, c.html, c.rtf, c.files, c.concealed = "", nil, "", "", nil, false
}

// setTextLocked stores text and notifies. The caller holds c.mu.
func (c *Clipboard) setTextLocked(text string, concealed bool) {
	c.clearLocked()
	c.text = text
	c.concealed = concealed
	c.notifyLocked()
}

// notifyLocked sends one coalesced change notification. The caller holds c.mu.
func (c *Clipboard) notifyLocked() {
	c.revision++
	select {
	case c.changed <- struct{}{}:
	default:
	}
}

var _ platform.ConcealedClipboard = (*Clipboard)(nil)
