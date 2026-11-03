package design

import (
	"fmt"
	"math"
)

// WCAG 2.1 contrast thresholds for text, as ratios of relative luminance.
const (
	ContrastAANormal = 4.5 // text under 18 pt, or under 14 pt bold
	ContrastAALarge  = 3.0 // text of at least 18 pt, or 14 pt bold
)

// Color is an sRGB color with straight (not premultiplied) alpha. It is a plain value, so the
// contrast math runs on tokens from any package without importing a toolkit.
type Color struct {
	R, G, B, A uint8
}

// Over composites top over under and returns the color the eye sees. Translucent text is measured as
// Over(text, surface).
func Over(top, under Color) Color {
	topAlpha := float64(top.A) / 255
	underAlpha := float64(under.A) / 255
	outAlpha := topAlpha + underAlpha*(1-topAlpha)
	if outAlpha == 0 {
		return Color{}
	}
	return Color{
		R: blendChannel(top.R, under.R, topAlpha, underAlpha, outAlpha),
		G: blendChannel(top.G, under.G, topAlpha, underAlpha, outAlpha),
		B: blendChannel(top.B, under.B, topAlpha, underAlpha, outAlpha),
		A: uint8(math.Round(outAlpha * 255)),
	}
}

func blendChannel(top, under uint8, topAlpha, underAlpha, outAlpha float64) uint8 {
	value := (float64(top)*topAlpha + float64(under)*underAlpha*(1-topAlpha)) / outAlpha
	return uint8(math.Round(value))
}

// Luminance returns the WCAG 2.1 relative luminance of c, from 0 (black) to 1 (white). The alpha is
// ignored: composite a translucent color with Over first.
func Luminance(c Color) float64 {
	return 0.2126*linearChannel(c.R) + 0.7152*linearChannel(c.G) + 0.0722*linearChannel(c.B)
}

func linearChannel(value uint8) float64 {
	srgb := float64(value) / 255
	if srgb <= 0.03928 {
		return srgb / 12.92
	}
	return math.Pow((srgb+0.055)/1.055, 2.4)
}

// Contrast returns the WCAG 2.1 contrast ratio of fg on bg, from 1 to 21. fg may be translucent: it is
// composited over bg first. bg must be opaque; composite a translucent surface over its backdrop with
// Over before calling, so the backdrop is explicit.
func Contrast(fg, bg Color) float64 {
	if bg.A != 255 {
		panic(fmt.Sprintf("design: background %+v must be opaque; composite it over its backdrop with Over", bg))
	}
	textLuminance := Luminance(Over(fg, bg))
	backgroundLuminance := Luminance(bg)
	lighter := math.Max(textLuminance, backgroundLuminance)
	darker := math.Min(textLuminance, backgroundLuminance)
	return (lighter + 0.05) / (darker + 0.05)
}

// AA reports whether fg on bg meets WCAG 2.1 AA: 4.5:1 for normal text, 3:1 when large is true.
func AA(fg, bg Color, large bool) bool {
	threshold := ContrastAANormal
	if large {
		threshold = ContrastAALarge
	}
	return Contrast(fg, bg) >= threshold
}
