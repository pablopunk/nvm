package app

import (
	"encoding/base64"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Bounds of an extension image. A data URL longer than maxImageDataURLBytes, or a file larger than maxImageFileBytes,
// is not decoded: the view shows its fallback instead.
const (
	maxImageDataURLBytes = 4 << 20
	maxImageFileBytes    = 8 << 20
)

// loadExtensionBitmap decodes an image an extension names: a base64 data URL or a local file, read in a bounded way.
// It runs off the UI thread. A nil result means the image is not shown.
func loadExtensionBitmap(files platform.FS, ref string) *ui.Bitmap {
	data, ok := imageBytes(files, ref)
	if !ok {
		return nil
	}
	bitmap, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	return bitmap
}

func imageBytes(files platform.FS, ref string) ([]byte, bool) {
	if rest, found := strings.CutPrefix(ref, "data:"); found {
		_, payload, hasPayload := strings.Cut(rest, "base64,")
		if !hasPayload || len(payload) > maxImageDataURLBytes {
			return nil, false
		}
		data, err := base64.StdEncoding.DecodeString(payload)
		return data, err == nil
	}
	if files == nil {
		return nil, false
	}
	data, size, err := files.ReadFilePrefix(ref, maxImageFileBytes)
	if err != nil || size > maxImageFileBytes {
		return nil, false
	}
	return data, true
}
