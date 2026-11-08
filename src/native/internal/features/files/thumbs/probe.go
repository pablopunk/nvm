package thumbs

import (
	"bytes"
	"image"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// probePrefixBytes is how much of a file Probe reads to see its image header.
const probePrefixBytes = 256 << 10

// Probe reports whether path holds a picture Render can draw, without decoding or scaling it. It reads only the start
// of the file and checks the image header, so a grid of many files is vetted in microseconds each while the palette
// draws the visible tiles lazily, as the 1.x host did.
func Probe(fsys platform.FS, path string) error {
	data, size, err := fsys.ReadFilePrefix(path, probePrefixBytes)
	if err != nil {
		return err
	}
	if size > MaxSourceBytes {
		return ErrTooLarge
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return ErrUnsupported
	}
	if config.Width > MaxSourcePixels/config.Height {
		return ErrTooLarge
	}
	return nil
}
