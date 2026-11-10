package extview

import (
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// GridImageRequest is host-owned view scope, never a wire payload supplied by an extension.
type GridImageRequest struct {
	ExtensionID     string
	Identity        any
	Source          string
	Command         *api.ActionRef
	Generation      uint64
	SnapshotVersion uint64
}

type GridImageResult struct {
	Bitmap     *ui.Bitmap
	Denial     *api.SnapshotDenial
	Authorized func() bool
}
