package platform

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"sync"

	"github.com/egoist/mygo/transfer"
)

// FileDrag is the intent "carry files out of the palette to another application as a native drag". A press on a row
// or grid cell that drags starts it: the returned transfer holds one file URL per path, and the OS drag shows preview.
// The real implementation is MyGo's transfer.FileData, which macOS, Linux and Windows each hand to their native drag
// (see platform/real). The fake records each call.
type FileDrag interface {
	StartFileDrag(paths []string, preview image.Image) (transfer.Data, error)
}

// FileDragSize is the side, in pixels, of a drag image that came from a file: the 64 pixel icon 1.x's drag uses.
const FileDragSize = 64

// genericFileDragPNG is the 48 pixel file icon 1.x draws for a file that has no picture of its own, such as a folder
// or an application bundle.
const genericFileDragPNG = "iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAIAAADYYG7QAAAABnRSTlMAAAAAAABupgeRAAAA9klEQVRYw2NgGAX4ASNBFdlFDdS1cmofPgPxOYjqTiHGWSzEaBYREVKUl6XcEfcfPn7z5h1+NThDCB48gX7umuoq1AqY6zfvrN+0E08gMREMGyq6hoGBQVNdRURECI8CAg6iSkyRZCYT0ebQCYw6aNRBow6iNSCq6oCD7XuOkGeNp4sNkSoHXQiNOogQIC0NEZ8UyAaDLoRGHUQIjJZDow6iFIyWQ6MOohTQqhwiO7UNuhAadRAhMFoOjTpo1EGjDkIF9x8+prqV+M0k4KA3b95dv3mHiq65fvMO/pFhogbOaTFOTdHA+Zs37wgOeFMLDLq5jkEHAG0sRE33acZ3AAAAAElFTkSuQmCC"

// fileDragPicture decodes its file on first use, when the OS drag reads the picture, so a row that is only drawn costs
// nothing.
type fileDragPicture struct {
	path string
	once sync.Once
	img  *image.NRGBA
}

// FileDragPreview returns the picture a drag of the file at path shows: the file itself, stretched to FileDragSize
// square when it is an image, else the 48 pixel generic file icon. It reads nothing until the picture is needed.
func FileDragPreview(path string) image.Image {
	return &fileDragPicture{path: path}
}

func (p *fileDragPicture) ColorModel() color.Model { return color.NRGBAModel }

func (p *fileDragPicture) Bounds() image.Rectangle { return p.picture().Bounds() }

func (p *fileDragPicture) At(x, y int) color.Color { return p.picture().At(x, y) }

func (p *fileDragPicture) picture() *image.NRGBA {
	p.once.Do(func() { p.img = loadFileDragPicture(p.path) })
	return p.img
}

func loadFileDragPicture(path string) *image.NRGBA {
	if img, ok := decodeFileImage(path); ok {
		return scaleToSquare(img, FileDragSize)
	}
	return toNRGBA(decodeGenericFileDragPNG())
}

func decodeFileImage(path string) (image.Image, bool) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	return img, err == nil
}

func decodeGenericFileDragPNG() image.Image {
	data, err := base64.StdEncoding.DecodeString(genericFileDragPNG)
	if err != nil {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	return img
}

// scaleToSquare resamples img to side × side, stretching it as 1.x's resize does, by nearest neighbour.
func scaleToSquare(img image.Image, side int) *image.NRGBA {
	src := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, side, side))
	if src.Dx() <= 0 || src.Dy() <= 0 {
		return out
	}
	for y := 0; y < side; y++ {
		sy := src.Min.Y + y*src.Dy()/side
		for x := 0; x < side; x++ {
			sx := src.Min.X + x*src.Dx()/side
			out.Set(x, y, img.At(sx, sy))
		}
	}
	return out
}

func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok {
		return n
	}
	out := image.NewNRGBA(img.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			out.Set(x, y, img.At(x, y))
		}
	}
	return out
}
