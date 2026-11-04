package app

import "github.com/egoist/mygo/ui"

// seed puts a decoded picture in the cache without a decode, for fixtures and goldens.
func (c *clipPictures) seed(name string, bitmap *ui.Bitmap) {
	c.ready[name] = bitmap
}
