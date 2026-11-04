package menubar

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// iconSize is the square the icon is drawn in, in pixels. The OS scales it to the menu bar.
const iconSize = 32

// iconRadius is the radius of the gold disc the placeholder icon draws. The final icon set is WP5.1.
const iconRadius = 12

// Icon returns the menu bar icon as PNG. It is a gold disc, the placeholder until the icon set ships.
func Icon() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	ink := color.NRGBA{R: 230, G: 180, B: 30, A: 255}
	center := float64(iconSize) / 2
	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			dx, dy := float64(x)+0.5-center, float64(y)+0.5-center
			if dx*dx+dy*dy <= iconRadius*iconRadius {
				img.Set(x, y, ink)
			}
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}
