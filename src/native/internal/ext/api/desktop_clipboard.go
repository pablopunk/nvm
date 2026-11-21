package api

// SystemClipboard reads and writes the OS clipboard directly. Concealed writes are kept out of
// Nevermind's history where the platform allows it; a clipboard that cannot keep a format out of history
// refuses a concealed write of that format rather than leaving it unmarked.
//
// Images come in two shapes: a PNG data URL for small uses, and a FileRef for a PNG the host wrote to a file, which
// extensions pass to rows (Item.File, Item.DragImage) and back to WriteImageFile without copying the bytes.
type SystemClipboard interface {
	ReadText() (string, error)
	WriteText(text string, concealed bool) error
	ReadHTML() (string, error)
	WriteHTML(html, text string, concealed bool) error
	// ReadRTF is read-only: the host has no RTF writer, so there is no WriteRTF.
	ReadRTF() (string, error)
	ReadImage() (dataURL string, found bool, err error)
	ReadImageFile() (image FileRef, found bool, err error)
	WriteImage(dataURL string, concealed bool) error
	WriteImageFile(path string, concealed bool) error
	ReadFiles() ([]FileRef, error)
	WriteFiles(paths []string, concealed bool) error
	// Read returns the first of the formats (text, html, rtf, image, files) that the clipboard holds, in the order
	// given. Type is "empty" when none of them is there. Nil or empty formats mean every format.
	Read(formats []string) (ClipboardContent, error)
	// Write puts one ClipboardContent on the clipboard, replacing what was there.
	Write(content ClipboardContent) error
}
