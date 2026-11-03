package app

import (
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// dragFileRow draws a result row that drags its files out to other applications: the row sits in a column that offers
// the paths as a file drag. Where the drag cannot be offered, the row is drawn without it. Drag-out has not been run
// on a desktop yet; it is verified by the rig and on macOS and Windows runners (see the WP1.7b report).
func dragFileRow(c *ui.Context, paths []string, draw func()) {
	data, err := transfer.FileData(paths...)
	if err != nil {
		draw()
		return
	}
	ui.Column(c).DragData(data).Children(draw)
}
