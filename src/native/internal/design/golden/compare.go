package golden

import (
	"image"
	"image/color"
)

var differingColor = color.RGBA{R: 255, G: 0, B: 0, A: 255}

// diffPixels compares two frames of the same size. A pixel differs when any channel differs by more than
// threshold. It returns a diff image: differing pixels in red, the rest of the actual frame faded toward
// white, so the changed regions stand out where they are.
func diffPixels(golden, actual *image.RGBA, threshold uint8) (*image.RGBA, int) {
	bounds := actual.Bounds()
	diff := image.NewRGBA(bounds)
	differing := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			want := golden.RGBAAt(x, y)
			got := actual.RGBAAt(x, y)
			if channelsDiffer(want, got, threshold) {
				differing++
				diff.SetRGBA(x, y, differingColor)
				continue
			}
			diff.SetRGBA(x, y, fadedTowardWhite(got))
		}
	}
	return diff, differing
}

func channelsDiffer(a, b color.RGBA, threshold uint8) bool {
	return channelDistance(a.R, b.R) > int(threshold) ||
		channelDistance(a.G, b.G) > int(threshold) ||
		channelDistance(a.B, b.B) > int(threshold) ||
		channelDistance(a.A, b.A) > int(threshold)
}

func channelDistance(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

func fadedTowardWhite(c color.RGBA) color.RGBA {
	return color.RGBA{R: fadeChannel(c.R), G: fadeChannel(c.G), B: fadeChannel(c.B), A: 255}
}

func fadeChannel(v uint8) uint8 {
	return uint8(255 - (255-int(v))/3)
}
