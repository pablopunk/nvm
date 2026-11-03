package real

import (
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// clipboard reads and writes the system clipboard through MyGo. Change notification polls the clipboard
// because MyGo offers no change event; see clipboard_watch.go.
type clipboard struct {
	clock     platform.Clock
	startOnce sync.Once
	changed   chan struct{}
}

// newClipboard returns the production clipboard port. The clock drives change polling.
func newClipboard(clock platform.Clock) *clipboard {
	return &clipboard{clock: clock, changed: make(chan struct{}, 1)}
}

// ReadText returns the plain text on the clipboard.
func (c *clipboard) ReadText() (string, error) {
	return mygo.Clipboard.ReadText(), nil
}

// WriteText replaces the clipboard with plain text.
func (c *clipboard) WriteText(text string) error {
	mygo.Clipboard.WriteText(text)
	return nil
}

// ReadImagePNG returns the clipboard image, or ErrNotFound when there is none.
func (c *clipboard) ReadImagePNG() ([]byte, error) {
	data := mygo.Clipboard.ReadImage()
	if len(data) == 0 {
		return nil, platform.ErrNotFound
	}
	return data, nil
}

// WriteImagePNG replaces the clipboard with a PNG image.
func (c *clipboard) WriteImagePNG(png []byte) error {
	return mygo.Clipboard.WriteImage(png)
}

// Formats lists the content kinds currently on the clipboard.
func (c *clipboard) Formats() ([]platform.ClipboardFormat, error) {
	var formats []platform.ClipboardFormat
	for _, format := range mygo.Clipboard.Formats() {
		switch format {
		case transfer.Text:
			formats = append(formats, platform.FormatText)
		case transfer.PNG:
			formats = append(formats, platform.FormatPNG)
		}
	}
	if isConcealedClipboard(mygo.Clipboard.AvailableFormats()) {
		formats = append(formats, platform.FormatConcealed)
	}
	return formats, nil
}

// Clear empties the clipboard.
func (c *clipboard) Clear() error {
	mygo.Clipboard.Clear()
	return nil
}

// Changed starts change polling on first use and returns the notification channel.
func (c *clipboard) Changed() <-chan struct{} {
	c.startOnce.Do(func() { go c.pollChanges() })
	return c.changed
}
