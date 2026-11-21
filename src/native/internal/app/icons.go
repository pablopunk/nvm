package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/design"
)

// iconSize is the point size of a row's system icon. The picture is asked for at twice that size, for the display scale.
const iconSize = design.FileIconSize

// fileIcon returns the picture a row shows for path: a thumbnail for an image once it is ready, otherwise the system
// icon of the path's kind. It never blocks; a miss queues the load and the redraw that follows shows the result.
func (p *App) fileIcon(path string) (*ui.Bitmap, bool) {
	return p.files.rowPicture(path)
}
