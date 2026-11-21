package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// clipboardCall is one host call of the desktop.clipboard service.
func (c remoteSystemClipboard) clipboardCall(op string, args, result any) error {
	return c.link.Call(protocol.ServiceDesktopClipboard, op, args, result)
}

func (c remoteSystemClipboard) ReadHTML() (string, error) {
	var html string
	err := c.clipboardCall(protocol.OpReadHTML, nil, &html)
	return html, err
}

func (c remoteSystemClipboard) WriteHTML(html, text string, concealed bool) error {
	return c.clipboardCall(protocol.OpWriteHTML, protocol.WriteHTMLArgs{HTML: html, Text: text, Concealed: concealed}, nil)
}

func (c remoteSystemClipboard) ReadRTF() (string, error) {
	var rtf string
	err := c.clipboardCall(protocol.OpReadRTF, nil, &rtf)
	return rtf, err
}

func (c remoteSystemClipboard) ReadImageFile() (api.FileRef, bool, error) {
	var result protocol.ImageFileResult
	err := c.clipboardCall(protocol.OpReadImageFile, nil, &result)
	return result.File, result.Found, err
}

func (c remoteSystemClipboard) WriteImageFile(path string, concealed bool) error {
	return c.clipboardCall(protocol.OpWriteImageFile, protocol.WriteImageFileArgs{Path: path, Concealed: concealed}, nil)
}

func (c remoteSystemClipboard) ReadFiles() ([]api.FileRef, error) {
	var files []api.FileRef
	err := c.clipboardCall(protocol.OpReadFiles, nil, &files)
	return files, err
}

func (c remoteSystemClipboard) WriteFiles(paths []string, concealed bool) error {
	return c.clipboardCall(protocol.OpWriteFiles, protocol.WriteFilesArgs{Paths: paths, Concealed: concealed}, nil)
}

func (c remoteSystemClipboard) Read(formats []string) (api.ClipboardContent, error) {
	var content api.ClipboardContent
	err := c.clipboardCall(protocol.OpReadFormats, protocol.FormatsArgs{Formats: formats}, &content)
	return content, err
}

func (c remoteSystemClipboard) Write(content api.ClipboardContent) error {
	return c.clipboardCall(protocol.OpWriteContent, protocol.ContentArgs{Content: content}, nil)
}
