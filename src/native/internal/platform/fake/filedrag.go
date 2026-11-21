package fake

import (
	"image"
	"sync"

	"github.com/egoist/mygo/transfer"
)

// FileDragCall is one request to start a file drag: the paths it carries and the image it shows.
type FileDragCall struct {
	Paths   []string
	Preview image.Image
}

// FileDrag is a platform.FileDrag that records each call and returns the transfer the real port would.
type FileDrag struct {
	mu    sync.Mutex
	calls []FileDragCall
}

// NewFileDrag returns a fake with no calls.
func NewFileDrag() *FileDrag {
	return &FileDrag{}
}

// StartFileDrag records the call and returns the file transfer for paths.
func (f *FileDrag) StartFileDrag(paths []string, preview image.Image) (transfer.Data, error) {
	f.mu.Lock()
	f.calls = append(f.calls, FileDragCall{Paths: append([]string(nil), paths...), Preview: preview})
	f.mu.Unlock()
	return transfer.FileData(paths...)
}

// Calls returns the calls so far, in order.
func (f *FileDrag) Calls() []FileDragCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]FileDragCall(nil), f.calls...)
}
