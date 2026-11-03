package platform

// ClipboardFormat names a kind of clipboard content, independent of the OS pasteboard type.
type ClipboardFormat string

// Clipboard formats the app distinguishes.
const (
	// FormatText is plain text.
	FormatText ClipboardFormat = "text"
	// FormatPNG is a PNG image.
	FormatPNG ClipboardFormat = "image/png"
	// FormatConcealed marks content a password manager or similar tool asked the OS not to record.
	// Clipboard history must never store or show it.
	FormatConcealed ClipboardFormat = "concealed"
)

// Clipboard is the intent "read and put content on the system clipboard, and notice when it changes".
// Writes replace everything on the clipboard, as the OS does. Image support is capability
// clipboard.image; change notification is capability clipboard.changed.
type Clipboard interface {
	// ReadText returns the plain text on the clipboard, or "" when there is none.
	ReadText() (string, error)
	// WriteText replaces the clipboard with plain text.
	WriteText(text string) error
	// ReadImagePNG returns the clipboard image as PNG, or ErrNotFound when there is no image.
	ReadImagePNG() ([]byte, error)
	// WriteImagePNG replaces the clipboard with a PNG image.
	WriteImagePNG(png []byte) error
	// Formats lists the kinds of content currently on the clipboard.
	Formats() ([]ClipboardFormat, error)
	// Clear empties the clipboard.
	Clear() error
	// Changed delivers one value per change, coalescing bursts. It returns nil where the
	// clipboard.changed capability is unsupported; check Capabilities before receiving from it.
	Changed() <-chan struct{}
}
