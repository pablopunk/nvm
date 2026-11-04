package app

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/features/clipboard"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// clipPreviewEdge is the longest side, in pixels, a picture is decoded to for the detail pane. Larger pictures
// are reduced before they reach the UI thread, so the pane's memory is bounded.
const clipPreviewEdge = 1024

// picLoadName names the background decode for panic reporting.
const picLoadName = "clipboard.picture.load"

// picState says where a picture stands in the cache.
type picState int

// Picture states. A picture is loading until its decode settles, then ready or failed.
const (
	picLoading picState = iota
	picReady
	picFailed
)

// clipPictures decodes the pictures of the detail pane off the UI thread and caches them. Its maps are touched
// on the UI thread only: bitmap reads them there, and each decode's result comes back through redraw, which runs
// the update on the UI thread. The decode itself (file read, decode, downscale) runs on a background goroutine.
type clipPictures struct {
	images  *clipboard.ImageStore
	redraw  func(fn func())
	decode  func(data []byte) (*ui.Bitmap, error)
	ready   map[string]*ui.Bitmap
	failed  map[string]bool
	loading map[string]bool
}

// newClipPictures returns an empty cache over the image store. decode turns stored bytes into a bitmap.
func newClipPictures(images *clipboard.ImageStore, redraw func(fn func()), decode func([]byte) (*ui.Bitmap, error)) *clipPictures {
	return &clipPictures{
		images:  images,
		redraw:  redraw,
		decode:  decode,
		ready:   map[string]*ui.Bitmap{},
		failed:  map[string]bool{},
		loading: map[string]bool{},
	}
}

// decodePicture is the production decode: a bounded PNG preview, then the bitmap the palette draws.
func decodePicture(data []byte) (*ui.Bitmap, error) {
	png, err := clipboard.PreviewPNG(data, clipPreviewEdge)
	if err != nil {
		return nil, err
	}
	bitmap, err := ui.DecodeBitmap(png)
	if err != nil {
		return nil, err
	}
	return bitmap, nil
}

// bitmap returns the decoded picture named name. When it is not decoded yet, it starts the decode in the
// background and returns nil with picLoading; the redraw after the decode shows it. It never decodes on the
// calling goroutine. An empty name is a picture that does not exist.
func (c *clipPictures) bitmap(name string) (*ui.Bitmap, picState) {
	if name == "" {
		return nil, picFailed
	}
	if bitmap, ok := c.ready[name]; ok {
		return bitmap, picReady
	}
	if c.failed[name] {
		return nil, picFailed
	}
	if !c.loading[name] {
		c.loading[name] = true
		c.start(name)
	}
	return nil, picLoading
}

// start runs the decode of name on a background goroutine and hands the result to the UI thread.
func (c *clipPictures) start(name string) {
	safego.Go(picLoadName, func(context.Context) {
		bitmap, err := c.load(name)
		c.redraw(func() {
			delete(c.loading, name)
			if err != nil {
				c.failed[name] = true
				return
			}
			c.ready[name] = bitmap
		})
	})
}

// load reads and decodes a stored picture. It runs off the UI thread.
func (c *clipPictures) load(name string) (*ui.Bitmap, error) {
	data, err := c.images.Read(name)
	if err != nil {
		return nil, err
	}
	return c.decode(data)
}

// seed puts a decoded picture in the cache without a decode, for fixtures and goldens.
func (c *clipPictures) seed(name string, bitmap *ui.Bitmap) {
	c.ready[name] = bitmap
}
