package extview

import (
	"encoding/json"
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/picture"
)

// GridImageRequest is host-owned view scope, never a wire payload supplied by an extension.
type GridImageRequest struct {
	Scope           *picture.Scope
	Session         *byte
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

func (r GridImageRequest) Binding() picture.Binding {
	var invocation string
	if r.Command != nil {
		data, _ := json.Marshal(r.Command)
		invocation = string(data)
	}
	return picture.Binding{ExtensionID: r.ExtensionID, Identity: r.Identity, Session: r.Session, Invocation: invocation, Generation: r.Generation, Version: r.SnapshotVersion}
}
