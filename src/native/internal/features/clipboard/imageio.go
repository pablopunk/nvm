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

// PreviewPNG decodes a PNG, JPEG or GIF picture within the decode limits, reduces it so its longer side is at
// most maxEdge pixels, and returns it as PNG. It is the one path pictures take to a preview: the link fetcher
// and the clipboard detail pane both use it, and neither runs it on the UI thread.
func PreviewPNG(data []byte, maxEdge int) ([]byte, error) {
	img, err := decodeLimited(data)
	if err != nil {
		return nil, err
	}
	return encodePNG(fitWithin(img, maxEdge))
}

// fitWithin returns img reduced by nearest-neighbour sampling so that its longer side is at most maxEdge
// pixels, keeping the aspect ratio. A picture already that small is returned as it is.
func fitWithin(img image.Image, maxEdge int) image.Image {
	bounds := img.Bounds()
	if maxEdge <= 0 || max(bounds.Dx(), bounds.Dy()) <= maxEdge {
		return img
	}
	width, height := bounds.Dx(), bounds.Dy()
	if width >= height {
		height = max(1, height*maxEdge/width)
		width = maxEdge
	} else {
		width = max(1, width*maxEdge/height)
		height = maxEdge
	}
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		sourceY := bounds.Min.Y + y*bounds.Dy()/height
		for x := range width {
			dst.Set(x, y, img.At(bounds.Min.X+x*bounds.Dx()/width, sourceY))
		}
	}
	return dst
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
