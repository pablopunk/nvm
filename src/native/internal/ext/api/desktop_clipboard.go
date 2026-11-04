package api

// SystemClipboard reads and writes the OS clipboard directly. Concealed writes are kept out of
// Nevermind's history where the platform allows it.
type SystemClipboard interface {
	ReadText() (string, error)
	WriteText(text string, concealed bool) error
	ReadHTML() (string, error)
	WriteHTML(html, text string, concealed bool) error
	ReadImage() (dataURL string, found bool, err error)
	WriteImage(dataURL string, concealed bool) error
	ReadFiles() ([]FileRef, error)
	WriteFiles(paths []string, concealed bool) error
	Read(formats []string) (ClipboardContent, error)
	Write(content ClipboardContent) error
}
