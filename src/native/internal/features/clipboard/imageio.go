package clipboard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/draw"
	"image/png"

	// The decoders register themselves; the store reads the formats the OS and file manager copy.
	_ "image/gif"
	_ "image/jpeg"
)

// decodeLimited decodes PNG, JPEG or GIF data after checking the declared size, so a small file that claims a
// huge canvas is refused before any pixel memory is allocated.
func decodeLimited(data []byte) (image.Image, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("clipboard: read image header: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > MaxImagePixels {
		return nil, errImageTooLarge
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("clipboard: decode image: %w", err)
	}
	return img, nil
}

// encodePNG encodes a decoded image as PNG, the store's format.
func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("clipboard: encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// pixelHash returns the content hash of an image's size and pixels. Two encodings of one picture hash alike.
func pixelHash(img image.Image) string {
	bounds := img.Bounds()
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%dx%d:", bounds.Dx(), bounds.Dy())
	_, _ = h.Write(toNRGBA(img).Pix)
	return hex.EncodeToString(h.Sum(nil))[:HashLength]
}

// toNRGBA copies an image into a zero-origin NRGBA buffer.
func toNRGBA(img image.Image) *image.NRGBA {
	bounds := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), img, bounds.Min, draw.Src)
	return dst
}
