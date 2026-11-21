package design

import "github.com/egoist/mygo/ui"

// galleryRows shows the list row in each of its states, and the icon tile in each of its forms.
func galleryRows(c *ui.Context) {
	Section(c, "Row: default, selected, prominent")
	Row(c, RowData{Title: "Default row", Subtitle: "Subtitle in the tertiary ink", Tile: TileData{Glyph: "AB"}, Tone: "blue"})
	Row(c, RowData{Title: "Selected row", Subtitle: "Warm plate with a gold edge", Tile: TileData{Icon: "clipboard"}, Tone: "blue", Selected: true, ShowEnter: true})
	Row(c, RowData{Title: "Automate", Subtitle: "Prominent, accent tint", Tile: TileData{Icon: "zap"}, Tone: "yellow", Prominent: true})
	Row(c, RowData{Title: "Automate", Subtitle: "Prominent and selected", Tile: TileData{Icon: "zap"}, Tone: "yellow", Prominent: true, Selected: true, ShowEnter: true})
	Section(c, "Row: accessory, tinted title, section header")
	Row(c, RowData{Title: "Pinned clipboard item", Subtitle: "Copied 3m ago · Text", Accessory: "Pinned", Tile: TileData{Icon: "clipboard"}, Tone: "blue", TintTitle: true})
	Row(c, RowData{Section: "Commands", Title: "Tinted title", Subtitle: "Tone purple", Tile: TileData{Icon: "puzzle"}, Tone: "purple", TintTitle: true})
	Section(c, "Row: hover and long text")
	Row(c, RowData{Title: "Hovered row", Subtitle: "The hover golden moves the pointer here", Tile: TileData{Glyph: "HO"}})
	Row(c, RowData{
		Title:     "A very long title that keeps going past the end of the row so that it must end in an ellipsis",
		Subtitle:  "A subtitle that is also much longer than the row can show, and must be cut rather than wrapped",
		Accessory: "Accessory",
		Tile:      TileData{Glyph: "LT"},
	})
	Section(c, "Row: picture and file icon")
	Row(c, RowData{Title: "Image preview", Subtitle: "Image 64×64", Tile: TileData{Image: galleryImage()}})
	Row(c, RowData{Title: "File icon", Subtitle: "The system icon of a file", Tile: TileData{FileIcon: galleryFileIcon()}})
	Section(c, "Tile: the forms of the icon tile")
	ui.Row(c).Gap(10).Children(func() {
		Tile(c, TileData{Icon: "search"})
		Tile(c, TileData{Glyph: "GH", Tone: "green"})
		Tile(c, TileData{Icon: "zap", Tone: "yellow", Prominent: true})
		Tile(c, TileData{Image: galleryImage()})
		Tile(c, TileData{FileIcon: galleryFileIcon()})
	})
}
