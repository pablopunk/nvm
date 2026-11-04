package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// ClipboardImages is the host's picture side of the clipboard: the files of stored pictures and of pictures an
// extension reads, and the thumbnails of rows.
type ClipboardImages interface {
	// Handle writes PNG bytes to a host-owned file and returns its path.
	Handle(png []byte) (string, error)
	// Path returns the PNG file of a stored history picture. It is valid while the entry stays in history.
	Path(ref string) (string, error)
	// Thumbnail returns a small PNG data URL of a stored history picture, or "" when there is none.
	Thumbnail(ref string) string
	// Preview returns the PNG data URL of a stored history picture at preview size, or "" when there is none.
	Preview(ref string) string
}

// maxClipboardFiles bounds the file list of one clipboard write from an extension.
const maxClipboardFiles = 1000

// defaultReadFormats is the order Read looks in when the extension names no formats: files before pictures before text,
// as a file manager copy also carries its path as text.
var defaultReadFormats = []string{"files", "image", "html", "rtf", "text"}

// serveClipboardFormats serves the clipboard formats beyond plain text: HTML, RTF, file lists, image files and the
// one-call Read and Write. Each op is one format, so a formats the platform cannot read is absent rather than an error.
func (h *Handler) serveClipboardFormats(call protocol.ServiceCall) (any, error) {
	switch call.Op {
	case protocol.OpReadHTML:
		return h.readClipboardHTML()
	case protocol.OpWriteHTML:
		return nil, h.writeClipboardHTML(call.Args)
	case protocol.OpReadRTF:
		return h.readClipboardRTF()
	case protocol.OpReadImageFile:
		return h.readClipboardImageFile()
	case protocol.OpWriteImageFile:
		return nil, h.writeClipboardImageFile(call.Args)
	case protocol.OpReadFiles:
		return h.readClipboardFiles()
	case protocol.OpWriteFiles:
		return nil, h.writeClipboardFiles(call.Args)
	case protocol.OpReadFormats:
		return h.readClipboardFormats(call.Args)
	case protocol.OpWriteContent:
		return nil, h.writeClipboardContent(call.Args)
	}
	return nil, invalid("desktop.clipboard has no operation %q", call.Op)
}

// richClipboard returns the clipboard's HTML, RTF and file-list port, or says the system cannot offer them.
func (h *Handler) richClipboard() (platform.RichClipboard, error) {
	if h.deps.Clipboard == nil {
		return nil, unavailable("the clipboard is not available in this build")
	}
	rich, ok := h.deps.Clipboard.(platform.RichClipboard)
	if !ok {
		return nil, unavailable("this system cannot read or write this clipboard format")
	}
	return rich, nil
}

func (h *Handler) readClipboardHTML() (string, error) {
	rich, err := h.richClipboard()
	if err != nil {
		return "", err
	}
	html, err := rich.ReadHTML()
	if err != nil {
		return "", fmt.Errorf("read the clipboard HTML: %w", err)
	}
	return html, nil
}

func (h *Handler) readClipboardRTF() (string, error) {
	rich, err := h.richClipboard()
	if err != nil {
		return "", err
	}
	rtf, err := rich.ReadRTF()
	if err != nil {
		return "", fmt.Errorf("read the clipboard RTF: %w", err)
	}
	return rtf, nil
}

func (h *Handler) writeClipboardHTML(raw []byte) error {
	var args protocol.WriteHTMLArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	return h.putHTML(args.HTML, args.Text, args.Concealed)
}

// putHTML writes HTML with its plain-text flavor. Concealed is refused: the rich writer cannot mark its content.
func (h *Handler) putHTML(html, text string, concealed bool) error {
	if len(html) > maxClipboardText || len(text) > maxClipboardText {
		return invalid("clipboard HTML is longer than %d bytes", maxClipboardText)
	}
	if concealed {
		return unavailable("this system cannot keep clipboard HTML out of history, so nothing was copied")
	}
	rich, err := h.richClipboard()
	if err != nil {
		return err
	}
	if !h.actions.allow() {
		return rateExceeded("clipboard writes")
	}
	return rich.WriteRich(text, html)
}

// readClipboardFiles returns the paths of the files on the clipboard, named by their last element.
func (h *Handler) readClipboardFiles() ([]api.FileRef, error) {
	rich, err := h.richClipboard()
	if err != nil {
		return nil, err
	}
	paths, err := rich.ReadFilePaths()
	if err != nil {
		return nil, fmt.Errorf("read the clipboard files: %w", err)
	}
	files := make([]api.FileRef, 0, len(paths))
	for _, path := range paths {
		files = append(files, api.FileRef{Path: path, Name: filepath.Base(path), Kind: "file"})
	}
	return files, nil
}

func (h *Handler) writeClipboardFiles(raw []byte) error {
	var args protocol.WriteFilesArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	return h.putFiles(args.Paths, args.Concealed)
}

// putFiles writes a file list. Every path must be absolute, so a relative path never means "the file next to the app".
func (h *Handler) putFiles(paths []string, concealed bool) error {
	if len(paths) == 0 || len(paths) > maxClipboardFiles {
		return invalid("a file list needs between 1 and %d paths", maxClipboardFiles)
	}
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			return invalid("clipboard file path %q is not absolute", path)
		}
	}
	if concealed {
		return unavailable("this system cannot keep clipboard files out of history, so nothing was copied")
	}
	rich, err := h.richClipboard()
	if err != nil {
		return err
	}
	if !h.actions.allow() {
		return rateExceeded("clipboard writes")
	}
	return rich.WriteFiles(paths)
}

// readClipboardImageFile writes the clipboard picture to a file the host owns, and returns it as a file handle.
func (h *Handler) readClipboardImageFile() (protocol.ImageFileResult, error) {
	png, found, err := h.clipboardPNG()
	if err != nil || !found {
		return protocol.ImageFileResult{}, err
	}
	if h.deps.ClipboardImages == nil {
		return protocol.ImageFileResult{}, unavailable("clipboard image files are not available in this build")
	}
	path, err := h.deps.ClipboardImages.Handle(png)
	if err != nil {
		return protocol.ImageFileResult{}, unavailable("the clipboard image cannot be written to a file: %v", err)
	}
	file := api.FileRef{Path: path, Name: filepath.Base(path), Kind: "image", Extension: "png", MimeType: "image/png", Size: int64(len(png))}
	return protocol.ImageFileResult{File: file, Found: true}, nil
}

// clipboardPNG reads the clipboard picture, reporting found false, without an error, when there is none.
func (h *Handler) clipboardPNG() ([]byte, bool, error) {
	if h.deps.Clipboard == nil {
		return nil, false, unavailable("the clipboard is not available in this build")
	}
	png, err := h.deps.Clipboard.ReadImagePNG()
	switch {
	case errors.Is(err, platform.ErrNotFound):
		return nil, false, nil
	case err != nil:
		return nil, false, unavailable("the clipboard image cannot be read: %v", err)
	case len(png) > maxClipboardImageBytes:
		return nil, false, unavailable("the clipboard image is larger than %d MB", maxClipboardImageBytes>>20)
	}
	return png, true, nil
}

func (h *Handler) writeClipboardImageFile(raw []byte) error {
	var args protocol.WriteImageFileArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	return h.putImageFile(args.Path, args.Concealed)
}

// putImageFile puts a PNG file on the clipboard. Only PNG files are accepted; the file is read at most once.
func (h *Handler) putImageFile(path string, concealed bool) error {
	if !filepath.IsAbs(path) || strings.ToLower(filepath.Ext(path)) != ".png" {
		return invalid("the image file must be an absolute PNG path")
	}
	if concealed {
		return unavailable("this clipboard cannot keep an image out of clipboard history")
	}
	png, err := os.ReadFile(path)
	if err != nil {
		return invalid("the image file cannot be read: %v", err)
	}
	return h.writeImageBytes(png)
}

// writeImageBytes puts PNG bytes on the clipboard, after the size check and the write limit.
func (h *Handler) writeImageBytes(png []byte) error {
	if len(png) > maxClipboardImageBytes {
		return invalid("the image is larger than %d MB", maxClipboardImageBytes>>20)
	}
	if h.deps.Clipboard == nil {
		return unavailable("the clipboard is not available in this build")
	}
	if !h.actions.allow() {
		return rateExceeded("clipboard writes")
	}
	if err := h.deps.Clipboard.WriteImagePNG(png); err != nil {
		return unavailable("the image cannot be put on the clipboard: %v", err)
	}
	return nil
}

// readClipboardFormats returns the first of the formats that the clipboard holds, in the order asked.
func (h *Handler) readClipboardFormats(raw []byte) (api.ClipboardContent, error) {
	var args protocol.FormatsArgs
	if err := decodeArgs(raw, &args); err != nil {
		return api.ClipboardContent{}, err
	}
	formats := args.Formats
	if len(formats) == 0 {
		formats = defaultReadFormats
	}
	for _, format := range formats {
		content, found, err := h.readFormat(format)
		if err != nil || found {
			return content, err
		}
	}
	return api.ClipboardContent{Type: "empty"}, nil
}

// readFormat reads one named format. A format the platform cannot read is not found, not an error.
func (h *Handler) readFormat(format string) (api.ClipboardContent, bool, error) {
	switch format {
	case "text":
		text, err := h.readClipboardText()
		return api.ClipboardContent{Type: "text", Text: text}, text != "", err
	case "html":
		return h.readHTMLFormat()
	case "rtf":
		rtf, err := h.readClipboardRTF()
		return api.ClipboardContent{Type: "text", RTF: rtf}, rtf != "", err
	case "image":
		return h.readImageFormat()
	case "files":
		return h.readFilesFormat()
	}
	return api.ClipboardContent{}, false, invalid("%q is not a clipboard format", format)
}

func (h *Handler) readHTMLFormat() (api.ClipboardContent, bool, error) {
	html, err := h.readClipboardHTML()
	if err != nil || html == "" {
		return api.ClipboardContent{}, false, err
	}
	text, _ := h.readClipboardText()
	return api.ClipboardContent{Type: "html", HTML: html, Text: text}, true, nil
}

func (h *Handler) readImageFormat() (api.ClipboardContent, bool, error) {
	image, err := h.readClipboardImage()
	if err != nil || !image.Found {
		return api.ClipboardContent{}, false, err
	}
	return api.ClipboardContent{Type: "image", ImageDataURL: image.DataURL}, true, nil
}

func (h *Handler) readFilesFormat() (api.ClipboardContent, bool, error) {
	files, err := h.readClipboardFiles()
	if err != nil || len(files) == 0 {
		return api.ClipboardContent{}, false, err
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	return api.ClipboardContent{Type: "files", Paths: paths}, true, nil
}

// writeClipboardContent puts one ClipboardContent on the clipboard. Concealed works for text only.
func (h *Handler) writeClipboardContent(raw []byte) error {
	var args protocol.ContentArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	content := args.Content
	switch content.Type {
	case "text":
		if content.HTML != "" {
			return h.putHTML(content.HTML, content.Text, content.Concealed)
		}
		return h.putText(content.Text, content.Concealed)
	case "html":
		return h.putHTML(content.HTML, content.Text, content.Concealed)
	case "image":
		return h.writeImageContent(content)
	case "files":
		return h.putFiles(content.Paths, content.Concealed)
	}
	return invalid("clipboard content type %q is not text, html, image or files", content.Type)
}

func (h *Handler) writeImageContent(content api.ClipboardContent) error {
	if content.ImagePath != "" {
		return h.putImageFile(content.ImagePath, content.Concealed)
	}
	if content.Concealed {
		return unavailable("this clipboard cannot keep an image out of clipboard history")
	}
	png, err := decodePNGDataURL(content.ImageDataURL)
	if err != nil {
		return err
	}
	return h.writeImageBytes(png)
}
