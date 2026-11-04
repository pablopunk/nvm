package platform

// ClipboardFormat names a kind of clipboard content, independent of the OS pasteboard type.
type ClipboardFormat string

// Clipboard formats the app distinguishes.
const (
	// FormatText is plain text.
	FormatText ClipboardFormat = "text"
	// FormatPNG is a PNG image.
	FormatPNG ClipboardFormat = "image/png"
	// FormatHTML is HTML markup next to plain text. Read it through RichClipboard.
	FormatHTML ClipboardFormat = "html"
	// FormatRTF is RTF markup next to plain text. Read it through RichClipboard.
	FormatRTF ClipboardFormat = "rtf"
	// FormatFiles is a list of file paths copied in the file manager. Read it through RichClipboard.
	FormatFiles ClipboardFormat = "files"
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

// ConcealedClipboard is an optional extension of Clipboard: text that clipboard history must never record. The
// dictation paste uses it so a transcript stays out of Clipboard History. A clipboard that cannot mark content as
// concealed does not implement it, and the caller refuses to put the text on the clipboard instead.
type ConcealedClipboard interface {
	// WriteConcealedText replaces the clipboard with plain text that carries the concealed marker.
	WriteConcealedText(text string) error
}

// RichClipboard is an optional extension of Clipboard for HTML, RTF and file lists. A clipboard that
// cannot read or write these formats does not implement it; callers type-assert and treat a missing
// implementation as "format absent", so plain-text behavior never depends on it.
type RichClipboard interface {
	// ReadHTML returns the HTML markup on the clipboard, or "" when there is none.
	ReadHTML() (string, error)
	// ReadRTF returns the RTF markup on the clipboard, or "" when there is none.
	ReadRTF() (string, error)
	// ReadFilePaths returns the absolute paths of the files on the clipboard, or nil.
	ReadFilePaths() ([]string, error)
	// WriteRich replaces the clipboard with plain text and its HTML markup, so apps that understand
	// HTML keep the formatting and others get the text.
	WriteRich(text, html string) error
	// WriteFiles replaces the clipboard with a list of absolute file paths, as a file manager copy does.
	WriteFiles(paths []string) error
}
