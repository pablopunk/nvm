package fake

import (
	"bytes"
	"slices"

	"github.com/pablopunk/nvm/native/internal/platform"
)

type clipboardSnapshot struct {
	text      string
	png       []byte
	html, rtf string
	files     []string
	concealed bool
}

type clipboardOwnership uint64

func (c *Clipboard) Snapshot() (platform.ClipboardSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return clipboardSnapshot{c.text, bytes.Clone(c.png), c.html, c.rtf, slices.Clone(c.files), c.concealed}, nil
}

func (c *Clipboard) WriteTemporaryText(text string) (platform.ClipboardOwnership, error) {
	if err := c.faults.take(OpClipboardWriteConcealed); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setTextLocked(text, true)
	return clipboardOwnership(c.revision), nil
}

func (c *Clipboard) StillOwns(ownership platform.ClipboardOwnership) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	owner, ok := ownership.(clipboardOwnership)
	return ok && uint64(owner) == c.revision, nil
}

func (c *Clipboard) RestoreIfOwned(snapshot platform.ClipboardSnapshot, ownership platform.ClipboardOwnership) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	want, ok := snapshot.(clipboardSnapshot)
	if !ok {
		return platform.ErrUnsupported
	}
	owner, ok := ownership.(clipboardOwnership)
	if !ok || uint64(owner) != c.revision {
		return nil
	}
	c.text, c.png, c.html, c.rtf, c.files, c.concealed = want.text, bytes.Clone(want.png), want.html, want.rtf, slices.Clone(want.files), want.concealed
	c.notifyLocked()
	return nil
}

var _ platform.ClipboardSnapshotter = (*Clipboard)(nil)
