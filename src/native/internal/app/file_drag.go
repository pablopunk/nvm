package app

import (
	"log"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// dragRow draws a row that drags its files out to other applications. The element it returns takes the press: a press
// that moves past MyGo's threshold starts the native drag and is no click, so the row is not chosen or run by it.
func (p *App) dragRow(c *ui.Context, src palette.DragSource, draw func()) ui.Element {
	target := ui.Column(c)
	port := p.ports.FileDrag
	if port == nil {
		target.Children(draw)
		return target
	}
	preview := platform.FileDragPreview(dragImagePath(src))
	target.DragDataFrom(func() transfer.Data {
		data, err := port.StartFileDrag(src.Paths, preview)
		if err != nil {
			log.Println("file drag:", err)
		}
		return data
	}, transfer.DragOptions{Preview: preview, Operations: transfer.Copy})
	target.Children(draw)
	return target
}

// dragImagePath names the file whose picture the drag shows: the source's image, else its first file.
func dragImagePath(src palette.DragSource) string {
	if src.Image != "" {
		return src.Image
	}
	return src.Paths[0]
}

// pathsOf returns the one file a row drags, or none when the row has no path.
func pathsOf(path string) []string {
	if path == "" {
		return nil
	}
	return []string{path}
}
