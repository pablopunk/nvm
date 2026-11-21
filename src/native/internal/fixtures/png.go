package fixtures

import (
	"bytes"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
)

// iconPNG draws a filled disc in a color derived from seed, on a transparent square, and encodes it as PNG.
// The same seed and size always produce the same bytes.
func iconPNG(seed string, size int) []byte {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(seed))
	sum := hash.Sum32()
	disc := color.NRGBA{R: byte(sum), G: byte(sum >> 8), B: byte(sum >> 16), A: 255}

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	radius := float64(size) / 2
	for y := range size {
		for x := range size {
			dx := float64(x) + 0.5 - radius
			dy := float64(y) + 0.5 - radius
			if dx*dx+dy*dy <= radius*radius {
				img.SetNRGBA(x, y, disc)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic("fixtures: encoding a generated icon failed: " + err.Error())
	}
	return buf.Bytes()
}
