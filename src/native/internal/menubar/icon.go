package menubar

import (
	"bytes"
	"image"
	"image/png"
)

// emojiTitle is the macOS title the emoji style shows in place of an image.
const emojiTitle = "⚡"

// IconChoice is the image an item shows: the glyph's file name and PNG, and whether macOS tints it to the bar.
// The emoji style on macOS has no glyph, so its choice is a transparent pixel with no asset name.
type IconChoice struct {
	Asset    string
	PNG      []byte
	Template bool
}

// ChooseIcon picks the image for style on goos (runtime.GOOS names) with the bar dark or light, as 1.x draws it.
// Template styles are black glyphs that macOS tints to the bar; elsewhere they are white on a dark bar and black on
// a light one. Colored styles always keep their colors.
func ChooseIcon(style, goos string, dark bool) IconChoice {
	resolved := resolveIconStyle(style)
	switch {
	case resolved.emojiTitle && goos == "darwin":
		return transparentChoice()
	case resolved.template && goos != "darwin" && dark:
		return newIconChoice(resolved.glyph+"-white", false)
	case resolved.template:
		return newIconChoice(resolved.glyph, goos == "darwin")
	default:
		return newIconChoice(resolved.glyph, false)
	}
}

// TitleFor is the title an item shows for title. On macOS the emoji style leads the title with its emoji, since the
// emoji is the item's whole image there.
func TitleFor(style, goos, title string) string {
	if !(resolveIconStyle(style).emojiTitle && goos == "darwin") {
		return title
	}
	if title == "" {
		return emojiTitle
	}
	return emojiTitle + " " + title
}

// newIconChoice loads the @2x file of a glyph; name is the glyph with its optional -white suffix.
func newIconChoice(name string, template bool) IconChoice {
	file := "nevermind-" + name + "@2x.png"
	return IconChoice{Asset: file, PNG: iconAsset(file), Template: template}
}

// transparentChoice replaces a glyph with an empty image. The platform keeps the previous icon for an empty PNG, so
// the glyph is cleared with a transparent pixel instead.
func transparentChoice() IconChoice {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	return IconChoice{PNG: buf.Bytes(), Template: true}
}
