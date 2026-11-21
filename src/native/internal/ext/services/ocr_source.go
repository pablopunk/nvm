package services

import (
	"encoding/base64"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// ocrSourceOf is the text an OCR input names: its source, else the file it carries, the way 1.x reads an ExtensionFile.
func ocrSourceOf(input api.OCRInput) string {
	switch {
	case input.Source != "":
		return input.Source
	case input.File == nil:
		return ""
	case input.File.Path != "":
		return input.File.Path
	case input.File.FileURL != "":
		return input.File.FileURL
	}
	return input.File.URL
}

// ocrPathOf is the image file a source names: the path of a file: URL, or a path whose leading "~" is the home folder.
func ocrPathOf(source, home string) string {
	if parsed, err := url.Parse(source); err == nil && parsed.Scheme == "file" {
		return fileURLPath(parsed)
	}
	if home != "" && (source == "~" || strings.HasPrefix(source, "~/")) {
		return filepath.Join(home, source[1:])
	}
	return source
}

// fileURLPath is the local path a file: URL names, the inverse of fileURLFor: a Windows drive path loses the slash its URL
// gives it.
func fileURLPath(parsed *url.URL) string {
	path := parsed.Path
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.FromSlash(path)
}

// ocrDataURLInput returns the image bytes a source carries as a base64 data URL of any image type, or false when it names
// a file. 1.x reads every image type, so a data URL is decoded as it comes.
func ocrDataURLInput(source string) ([]byte, bool, error) {
	if !strings.HasPrefix(source, "data:") {
		return nil, false, nil
	}
	mediaType, payload, found := strings.Cut(source, ";base64,")
	if !found || !strings.HasPrefix(mediaType, "data:image/") {
		return nil, true, invalid("OCR needs an image file path, a file URL or an image data URL")
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, true, invalid("the image data URL is not valid base64")
	}
	if len(data) > maxClipboardImageBytes {
		return nil, true, invalid("the image is larger than %d MB", maxClipboardImageBytes>>20)
	}
	return data, true, nil
}
