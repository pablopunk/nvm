package services

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// maxClipboardImageBytes bounds one clipboard image in either direction.
const maxClipboardImageBytes = 16 << 20

const pngDataURLPrefix = "data:image/png;base64,"

// readClipboardImage returns the clipboard's PNG as a data URL, or Found false when it holds no image.
func (h *Handler) readClipboardImage() (protocol.ImageResult, error) {
	if h.deps.Clipboard == nil {
		return protocol.ImageResult{}, unavailable("the clipboard is not available in this build")
	}
	png, err := h.deps.Clipboard.ReadImagePNG()
	switch {
	case errors.Is(err, platform.ErrNotFound):
		return protocol.ImageResult{}, nil
	case errors.Is(err, platform.ErrUnsupported):
		return protocol.ImageResult{}, unavailable("clipboard images are not available on this system")
	case err != nil:
		return protocol.ImageResult{}, unavailable("the clipboard image cannot be read: %v", err)
	case len(png) > maxClipboardImageBytes:
		return protocol.ImageResult{}, unavailable("the clipboard image is larger than %d MB", maxClipboardImageBytes>>20)
	}
	return protocol.ImageResult{DataURL: pngDataURLPrefix + base64.StdEncoding.EncodeToString(png), Found: true}, nil
}

// writeClipboardImage puts a PNG data URL on the clipboard. A clipboard that cannot keep an image out of history
// refuses a concealed write, as it refuses a concealed text write it cannot mark.
func (h *Handler) writeClipboardImage(raw []byte) error {
	var args protocol.WriteImageArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	if args.Concealed {
		return unavailable("this clipboard cannot keep an image out of clipboard history")
	}
	png, err := decodePNGDataURL(args.DataURL)
	if err != nil {
		return err
	}
	if h.deps.Clipboard == nil {
		return unavailable("the clipboard is not available in this build")
	}
	if err := h.deps.Clipboard.WriteImagePNG(png); err != nil {
		return unavailable("the image cannot be put on the clipboard: %v", err)
	}
	return nil
}

// decodePNGDataURL returns the bytes of a PNG data URL. Other data URLs and oversized images are invalid input.
func decodePNGDataURL(dataURL string) ([]byte, error) {
	if !strings.HasPrefix(dataURL, pngDataURLPrefix) {
		return nil, invalid("the image must be a PNG data URL")
	}
	png, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, pngDataURLPrefix))
	if err != nil {
		return nil, invalid("the image data URL is not valid base64")
	}
	if len(png) > maxClipboardImageBytes {
		return nil, invalid("the image is larger than %d MB", maxClipboardImageBytes>>20)
	}
	return png, nil
}
