package real

import (
	"image"

	"github.com/egoist/mygo/transfer"
)

// fileDrag offers files through MyGo. The window draws the drag image from the drag options the caller sets, so the
// transfer carries only the file URLs; macOS, Linux and Windows each hand them to their native drag.
type fileDrag struct{}

func (fileDrag) StartFileDrag(paths []string, _ image.Image) (transfer.Data, error) {
	return transfer.FileData(paths...)
}
