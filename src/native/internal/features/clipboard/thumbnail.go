package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
)

// ErrInvalidThumbnailSize refuses a thumbnail side of zero or less.
var ErrInvalidThumbnailSize = errors.New("clipboard: thumbnail size must be positive")

// Thumbnail returns a PNG whose longer side is at most maxSide pixels, keeping the aspect ratio. An image
// already within the limit is returned unchanged. The downscale averages each block of source pixels, a box
// filter that is cheap and needs no dependency.
func Thumbnail(data []byte, maxSide int) ([]byte, error) {
	if maxSide <= 0 {
		return nil, ErrInvalidThumbnailSize
	}
	img, err := decodeLimited(data)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	if max(bounds.Dx(), bounds.Dy()) <= maxSide {
		return data, nil
	}
	width, height := scaledSize(bounds.Dx(), bounds.Dy(), maxSide)
	var buf bytes.Buffer
	if err := png.Encode(&buf, boxDownscale(toNRGBA(img), width, height)); err != nil {
		return nil, fmt.Errorf("clipboard: encode thumbnail: %w", err)
	}
	return buf.Bytes(), nil
}

// scaledSize returns the size that fits a w by h image into maxSide on its longer side. The shorter side
// is at least one pixel.
func scaledSize(w, h, maxSide int) (int, int) {
	if w >= h {
		return maxSide, max(1, h*maxSide/w)
	}
	return max(1, w*maxSide/h), maxSide
}

// boxDownscale shrinks src to width by height. Each destination pixel is the average of the source pixels
// in its block; a block is never empty, so every destination pixel has a value.
func boxDownscale(src *image.NRGBA, width, height int) *image.NRGBA {
	srcW, srcH := src.Rect.Dx(), src.Rect.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		y0, y1 := blockRange(y, height, srcH)
		for x := range width {
			x0, x1 := blockRange(x, width, srcW)
			averageBlock(src, dst, x, y, x0, x1, y0, y1)
		}
	}
	return dst
}

// blockRange returns the source span [start, end) that maps onto destination index i of n, at least one
// source pixel wide.
func blockRange(i, n, source int) (int, int) {
	start := i * source / n
	end := max(start+1, (i+1)*source/n)
	return start, end
}

// averageBlock writes the mean of the source block [x0,x1) x [y0,y1) into dst at (x, y).
func averageBlock(src, dst *image.NRGBA, x, y, x0, x1, y0, y1 int) {
	var sum [4]uint64
	count := uint64(0)
	for sy := y0; sy < y1; sy++ {
		row := sy * src.Stride
		for sx := x0; sx < x1; sx++ {
			p := row + sx*4
			for c := range 4 {
				sum[c] += uint64(src.Pix[p+c])
			}
			count++
		}
	}
	offset := y*dst.Stride + x*4
	for c := range 4 {
		dst.Pix[offset+c] = uint8(sum[c] / count)
	}
}
