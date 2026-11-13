package real

import (
	"context"
	"os"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// rtfFormat is the RTF target. MyGo names no RTF format of its own, so the raw MIME type is asked for; the
// Linux backend offers any target the owner advertises, and other backends may return transfer.ErrFormat.
const rtfFormat transfer.Format = "text/rtf"

// clipboard reads and writes the system clipboard through MyGo. It offers no change notification: MyGo has
// no change event and the watcher polls adaptively (see features/clipboard), so Changed returns nil and the
// clipboard.changed capability is not reported. HTML, RTF and file lists are read and written through MyGo's
// portable formats; see rich_clipboard.go. On Wayland a client cannot read the clipboard while it has no focus, so
// the clipboard also is a platform.ClipboardFeed that delivers every copy through wl-paste (clipboard_wayland.go).
type clipboard struct {
	feed *waylandFeed
}

var _ platform.ClipboardFeed = (*clipboard)(nil)

// newClipboard returns the production clipboard port.
func newClipboard() *clipboard {
	return &clipboard{feed: newWaylandFeed(runtime.GOOS, os.Getenv, hasTool, startProcessStream)}
}

// WatchCopies delivers each copy with its content while the app is in the background. It returns a nil channel
// off Wayland, where the ordinary reads work.
func (c *clipboard) WatchCopies(ctx context.Context) (<-chan platform.ClipboardCopy, error) {
	return c.feed.WatchCopies(ctx)
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

// Formats lists the content kinds currently on the clipboard. Concealed is reported when the owner
// advertises one of the concealed markers in its raw format names (see isConcealedClipboard).
func (c *clipboard) Formats() ([]platform.ClipboardFormat, error) {
	var formats []platform.ClipboardFormat
	for _, format := range mygo.Clipboard.Formats() {
		if kind, ok := portableFormat(format); ok && !containsFormat(formats, kind) {
			formats = append(formats, kind)
		}
	}
	if isConcealedClipboard(mygo.Clipboard.AvailableFormats()) {
		formats = append(formats, platform.FormatConcealed)
	}
	return formats, nil
}

// portableFormat maps a MyGo format to the kinds the app distinguishes.
func portableFormat(format transfer.Format) (platform.ClipboardFormat, bool) {
	switch format {
	case transfer.Text:
		return platform.FormatText, true
	case transfer.PNG:
		return platform.FormatPNG, true
	case transfer.HTML:
		return platform.FormatHTML, true
	case rtfFormat:
		return platform.FormatRTF, true
	case transfer.FileList, transfer.URIList:
		return platform.FormatFiles, true
	}
	return "", false
}

// containsFormat reports whether list already holds kind.
func containsFormat(list []platform.ClipboardFormat, kind platform.ClipboardFormat) bool {
	for _, item := range list {
		if item == kind {
			return true
		}
	}
	return false
}

// Clear empties the clipboard.
func (c *clipboard) Clear() error {
	mygo.Clipboard.Clear()
	return nil
}

// Changed returns nil: this build has no native change notification (see the type comment).
func (c *clipboard) Changed() <-chan struct{} {
	return nil
}
