package design

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// TileSize is the side of a row's icon tile, in DIPs.
const TileSize = 32

// FileIconSize is the side a file or app icon is drawn at. System icons keep about a tenth of their canvas as
// transparent margin on each side, so drawn at this size their visible body matches a TileSize key beside them.
const FileIconSize = 40

// TileData is the icon of a row. The first of these that is set wins: a preview image, the file's own icon,
// a Lucide icon, then the one or two letters of Glyph (or of Title).
type TileData struct {
	Image *ui.Bitmap
	// ImageShape is the corners of Image: square, rounded (the default) or circle.
	ImageShape string
	FileIcon   *ui.Bitmap
	Icon       string
	Glyph      string
	Title      string
	// Tone tints a glyph or icon with the named tone; Prominent gives the tile the gold key.
	Tone      string
	Prominent bool
}

// ImageRadius is the corner radius of a picture of the given size with the named shape.
func ImageRadius(shape string, size float32) float32 {
	switch shape {
	case "square", "none":
		return 0
	case "circle":
		return size / 2
	}
	return 8
}

// Tile draws a row's icon: a 32 by 32 tile, either a picture or a small raised key with a tinted symbol.
func Tile(c *ui.Context, d TileData) {
	k := Tokens(c)
	switch {
	case d.Image != nil:
		ui.Image(c, d.Image).Size(TileSize, TileSize).Fit(ui.Cover).Radius(ImageRadius(d.ImageShape, TileSize)).Clip()
	case d.FileIcon != nil:
		ui.Row(c).Size(TileSize, TileSize).Center().Children(func() { ui.Image(c, d.FileIcon).Size(FileIconSize, FileIconSize) })
	default:
		fg := k.Secondary
		if col, ok := k.Tones[d.Tone]; ok {
			fg = col
		}
		key := ui.Row(c).Size(TileSize, TileSize).Center()
		if d.Prominent {
			Glossy(key, k.AccentFill, 8, 0)
			fg = k.AccentContrast
		} else {
			key.Radius(8).Background(k.Interactive).Border(1, k.EdgeSubtle).InsetShadow(0, 1, 0, 0, k.Hi)
		}
		key.Children(func() { tileSymbol(c, d, fg) })
	}
}

// tileSymbol draws the icon of a key, or its letters when it has none.
func tileSymbol(c *ui.Context, d TileData, fg ui.Color) {
	if d.Icon != "" {
		ui.Icon(c, Icon(d.Icon)).FontSize(17).TextColor(fg)
		return
	}
	glyph := d.Glyph
	if glyph == "" {
		glyph = d.Title
	}
	runes := []rune(strings.ToUpper(glyph))
	ui.Text(c, string(runes[:min(2, len(runes))])).FontSize(12).FontWeight(700).TextColor(fg)
}
