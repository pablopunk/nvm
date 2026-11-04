package real

import (
	"errors"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

// ReadHTML returns the HTML markup on the clipboard, or "" when there is none.
func (c *clipboard) ReadHTML() (string, error) {
	return mygo.Clipboard.ReadHTML(), nil
}

// ReadRTF returns the RTF markup on the clipboard, or "" when the owner offers none. Only the Linux backend
// is known to offer raw MIME targets through MyGo; the macOS and Windows paths are untested.
func (c *clipboard) ReadRTF() (string, error) {
	data, err := mygo.Clipboard.ReadFormat(rtfFormat)
	if err != nil {
		if errors.Is(err, transfer.ErrFormat) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// ReadFilePaths returns the absolute paths of the files on the clipboard, or nil when it holds none.
func (c *clipboard) ReadFilePaths() ([]string, error) {
	paths, err := mygo.Clipboard.ReadFiles()
	if err != nil {
		return nil, nil
	}
	return paths, nil
}

// WriteRich replaces the clipboard with plain text and HTML markup in one write.
func (c *clipboard) WriteRich(text, html string) error {
	return mygo.Clipboard.Write(transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.HTML, []byte(html)),
		transfer.Bytes(transfer.Text, []byte(text)),
	)))
}

// WriteFiles replaces the clipboard with the file list of paths.
func (c *clipboard) WriteFiles(paths []string) error {
	return mygo.Clipboard.WriteFiles(paths...)
}
