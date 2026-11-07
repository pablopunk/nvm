// Package thumbs makes small PNG pictures of image files for the Files view: a thumbnail for a list tile and a larger
// picture for the preview pane. Decoding and scaling run on the caller's goroutine, so the app calls them from its
// worker pool, never from the UI thread. Rendering follows no symbolic link, refuses files over MaxSourceBytes and
// reports the formats the standard library decodes (PNG, JPEG, GIF) as the only supported ones.
//
// Layer: features. It imports the platform ports and the standard library only.
package thumbs

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	_ "image/gif"  // registers the GIF decoder with image.Decode
	_ "image/jpeg" // registers the JPEG decoder with image.Decode

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Edge sizes, in pixels, of the list thumbnail and the preview picture. Both keep the aspect ratio inside a square.
const (
	Edge        = 64
	PreviewEdge = 1024
)

// MaxSourceBytes is the largest image rendered. A larger file reports ErrTooLarge and gets no picture.
const MaxSourceBytes = 25 << 20

// MaxSourcePixels bounds allocations during decoding.
const MaxSourcePixels = 32_000_000

// Errors a render reports. A caller treats each as "no picture" and says nothing to the user.
var (
	ErrUnsupported = errors.New("thumbs: the file is not an image this build decodes")
	ErrSymlink     = errors.New("thumbs: symbolic links are not followed")
	ErrTooLarge    = errors.New("thumbs: the image is over the size cap")
)

// Render decodes the image at path, scales it to fit an edge-by-edge square and returns it as PNG. It reads the path
// without following a final symbolic link, and it never reads more than MaxSourceBytes.
func Render(fsys platform.FS, path string, edge int) ([]byte, error) {
	reader, ok := fsys.(interface {
		ReadFileNoFollow(string, int64) ([]byte, error)
	})
	if !ok {
		return nil, ErrUnsupported
	}
	data, err := reader.ReadFileNoFollow(path, MaxSourceBytes)
	if errors.Is(err, platform.ErrFileSymlink) {
		return nil, ErrSymlink
	}
	if errors.Is(err, platform.ErrFileTooLarge) {
		return nil, ErrTooLarge
	}
	if err != nil {
		return nil, err
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return nil, ErrUnsupported
	}
	if config.Width > MaxSourcePixels/config.Height {
		return nil, ErrTooLarge
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrUnsupported
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, orientThumbnail(fitInside(src, edge), jpegOrientation(data))); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fitInside scales src to fit a square of edge pixels, keeping its aspect ratio. Pictures already inside the square are
// copied at their own size. Scaling samples the nearest source pixel, which is cheap and enough for a tile.
func fitInside(src image.Image, edge int) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	nw, nh := w, h
	if w > edge || h > edge {
		if w >= h {
			nw, nh = edge, max(1, h*edge/w)
		} else {
			nw, nh = max(1, w*edge/h), edge
		}
	}
	out := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	if nw == w && nh == h {
		draw.Draw(out, out.Bounds(), src, b.Min, draw.Src)
		return out
	}
	for y := range nh {
		sy := b.Min.Y + y*h/nh
		for x := range nw {
			sx := b.Min.X + x*w/nw
			out.SetNRGBA(x, y, color.NRGBAModel.Convert(src.At(sx, sy)).(color.NRGBA))
		}
	}
	return out
}
