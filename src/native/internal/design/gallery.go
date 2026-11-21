package design

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"

	"github.com/egoist/mygo/ui"
)

// GalleryWidth and GalleryHeight are the window the gallery goldens render in: the palette's width, and a
// height that fits a page of the gallery.
const (
	GalleryWidth  = 720
	GalleryHeight = 900
)

// Gallery shows every component of this package, each in the states it has, as one scrollable column. It is a
// development view: the design goldens render it, and no user ever opens it.
func Gallery(c *ui.Context) {
	Pane(c, func() {
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(16).Gap(12).Children(func() {
				galleryRows(c)
				galleryControls(c)
				galleryStates(c)
			})
		})
	})
}

// galleryPage wraps one page of the gallery in the pane and the scroll view it is shown with, so that a golden
// renders the page as the palette would hold it.
func galleryPage(page func(c *ui.Context)) func(c *ui.Context) {
	return func(c *ui.Context) {
		Pane(c, func() {
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(16).Gap(12).Children(func() { page(c) })
			})
		})
	}
}

// galleryState is what the gallery's inputs show. It is fixed, so every render of the gallery is the same.
var galleryState = struct {
	query, blank, single, multiline string
	searchInput, emptyInput         ui.Handle
}{
	query:     "clip",
	single:    "Email signature",
	multiline: "Text to paste, on one or more lines.",
}

var (
	galleryPhotoOnce sync.Once
	galleryPhoto     *ui.Bitmap
	galleryIconOnce  sync.Once
	galleryIcon      *ui.Bitmap
)

// galleryImage is a picture for the image tile: a warm block on a cool field, decoded once.
func galleryImage() *ui.Bitmap {
	galleryPhotoOnce.Do(func() { galleryPhoto = galleryBitmap(64, color.NRGBA{R: 96, G: 128, B: 160, A: 255}) })
	return galleryPhoto
}

// galleryFileIcon is a stand-in for the system icon of a file.
func galleryFileIcon() *ui.Bitmap {
	galleryIconOnce.Do(func() { galleryIcon = galleryBitmap(56, color.NRGBA{R: 230, G: 189, B: 50, A: 255}) })
	return galleryIcon
}

// galleryBitmap makes a square image of the given side, decoded the way the palette decodes real pictures.
func galleryBitmap(side int, fill color.NRGBA) *ui.Bitmap {
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	for y := side / 4; y < side/2; y++ {
		for x := side / 4; x < side/2; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 250, G: 248, B: 243, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	bm, err := ui.DecodeBitmap(buf.Bytes())
	if err != nil {
		return nil
	}
	return bm
}
