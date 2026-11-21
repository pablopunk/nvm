package clipboard

import (
	"errors"
	"image"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// candidate is one copy the watcher may store: the entry, and for images the PNG bytes to save.
type candidate struct {
	entry Entry
	png   []byte
}

// formatState is the clipboard's formats, read once per poll. known is false when the format list could
// not be read; the watcher then tries every format, as the 1.x code did when its marker check failed.
type formatState struct {
	formats map[platform.ClipboardFormat]bool
	known   bool
}

// readFormats reads the format list of the clipboard.
func (w *Watcher) readFormats() formatState {
	formats, err := w.deps.Clipboard.Formats()
	if err != nil {
		return formatState{}
	}
	set := make(map[platform.ClipboardFormat]bool, len(formats))
	for _, f := range formats {
		set[f] = true
	}
	return formatState{formats: set, known: true}
}

// has reports whether the clipboard holds the format.
func (s formatState) has(format platform.ClipboardFormat) bool {
	return s.formats[format]
}

// concealed reports whether a password manager or similar tool asked the OS not to record this copy.
func (s formatState) concealed() bool {
	return s.known && s.has(platform.FormatConcealed)
}

// hasImage reports whether an image read may find something.
func (s formatState) hasImage() bool {
	return !s.known || s.has(platform.FormatPNG)
}

// errImageTooLarge refuses an image with more pixels than MaxImagePixels.
var errImageTooLarge = errors.New("clipboard: image has too many pixels")

// readCandidate applies the capture rules to the clipboard, in the 1.x order: copied files first, then
// text, then an image when the caller asks for one. It returns nil when there is nothing to store.
func (w *Watcher) readCandidate(state formatState, readImage bool) *candidate {
	if w.rich != nil && (!state.known || state.has(platform.FormatFiles)) {
		if paths := w.readFilePaths(); len(paths) > 0 {
			return w.fileCandidate(paths)
		}
	}
	text := w.readText()
	if text != "" {
		if len(text) > MaxCaptureTextBytes {
			w.logDropped("text-too-large")
			return nil
		}
		return w.textCandidate(text, state)
	}
	if readImage {
		return w.imageCandidate()
	}
	return nil
}

// readText returns the trimmed plain text, or "" when there is none or it cannot be read.
func (w *Watcher) readText() string {
	text, err := w.deps.Clipboard.ReadText()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(text)
}

// readFilePaths returns the copied file paths, or nil.
func (w *Watcher) readFilePaths() []string {
	paths, err := w.rich.ReadFilePaths()
	if err != nil {
		return nil
	}
	return paths
}

// textCandidate keeps rich markup next to the plain text when the clipboard carries it, and otherwise
// stores the text alone.
func (w *Watcher) textCandidate(text string, state formatState) *candidate {
	if w.rich != nil {
		if html := w.readMarkup(platform.FormatHTML, state, w.rich.ReadHTML); html != "" {
			return &candidate{entry: Entry{ID: HTMLID(html), Kind: KindHTML, Text: text, Original: html}}
		}
		if rtf := w.readMarkup(platform.FormatRTF, state, w.rich.ReadRTF); rtf != "" {
			return &candidate{entry: Entry{ID: RTFID(rtf), Kind: KindRTF, Text: text, Original: rtf}}
		}
	}
	return &candidate{entry: Entry{ID: TextID(text), Kind: KindText, Text: text}}
}

// readMarkup reads one markup format when the clipboard holds it and the markup is within the size cap.
func (w *Watcher) readMarkup(format platform.ClipboardFormat, state formatState, read func() (string, error)) string {
	if state.known && !state.has(format) {
		return ""
	}
	markup, err := read()
	if err != nil || len(markup) > MaxCaptureTextBytes {
		return ""
	}
	return markup
}

// imageCandidate reads the clipboard PNG and checks its size and pixels before it is stored.
func (w *Watcher) imageCandidate() *candidate {
	png, err := w.deps.Clipboard.ReadImagePNG()
	if err != nil {
		if !errors.Is(err, platform.ErrNotFound) {
			w.deps.Log.Warn("clipboard.image.read.failed", logx.F{"error": err.Error()})
		}
		return nil
	}
	if len(png) > MaxCaptureImageBytes {
		w.logDropped("image-too-large")
		return nil
	}
	img, err := decodeLimited(png)
	if err != nil {
		w.logDropped("image-undecodable")
		return nil
	}
	return candidateFromImage(img, png)
}

// fileCandidate stores a single copied image file as an image, and any other list of files as a files
// entry. An image file that cannot be read falls back to the files entry.
func (w *Watcher) fileCandidate(paths []string) *candidate {
	if len(paths) == 1 && w.deps.Files != nil && isImagePath(paths[0]) {
		if c := w.imageFileCandidate(paths[0]); c != nil {
			return c
		}
	}
	return &candidate{entry: Entry{ID: FilesID(paths), Kind: KindFiles, Paths: paths, Text: strings.Join(paths, "\n")}}
}

// imageFileCandidate decodes a copied image file and re-encodes it as PNG, the store's format.
func (w *Watcher) imageFileCandidate(path string) *candidate {
	info, err := w.deps.Files.Stat(path)
	if err != nil || info.Size() > MaxCaptureImageBytes {
		return nil
	}
	data, err := w.deps.Files.ReadFile(path)
	if err != nil {
		return nil
	}
	img, err := decodeLimited(data)
	if err != nil {
		return nil
	}
	png, err := encodePNG(img)
	if err != nil {
		return nil
	}
	return candidateFromImage(img, png)
}

// candidateFromImage describes a decoded image: its pixel hash is its identity, so the same picture
// re-encoded by the OS keeps its entry.
func candidateFromImage(img image.Image, png []byte) *candidate {
	hash := pixelHash(img)
	bounds := img.Bounds()
	return &candidate{
		entry: Entry{ID: ImageID(hash), Kind: KindImage, Hash: hash, Width: bounds.Dx(), Height: bounds.Dy()},
		png:   png,
	}
}

// logDropped records why a copy was not stored. The reason is a fixed word; the content is never logged.
func (w *Watcher) logDropped(reason string) {
	w.deps.Log.Debug("clipboard.capture.dropped", logx.F{"reason": reason})
}

// isImagePath reports whether a file name has an extension the decoders read.
func isImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	}
	return false
}
