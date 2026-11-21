package onnxocr

import (
	"fmt"
	"image"
	_ "image/gif"  // decoders the file picker and screenshots can hand over
	_ "image/jpeg" // as above
	_ "image/png"  // as above
	"os"

	_ "golang.org/x/image/bmp" // as above
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff" // as above
	_ "golang.org/x/image/webp" // as above
)

func decodeImage(path string) (*image.RGBA, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("the image could not be read: %w", err)
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("the image could not be read: %w", err)
	}
	bounds := decoded.Bounds()
	picture := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(picture, picture.Bounds(), decoded, bounds.Min, draw.Src)
	return picture, nil
}

func resized(source image.Image, width, height int) *image.RGBA {
	target := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.BiLinear.Scale(target, target.Bounds(), source, source.Bounds(), draw.Src, nil)
	return target
}

// fillChannels writes picture into data as normalized blue, green and red planes, the order the models were trained
// on. stride is the width of the planes, which can be wider than the picture; the rest stays zero.
func fillChannels(data []float32, picture *image.RGBA, stride int) {
	bounds := picture.Bounds()
	plane := stride * bounds.Dy()
	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			pixel := picture.PixOffset(x, y)
			for channel, source := range [3]int{2, 1, 0} {
				data[channel*plane+y*stride+x] = (float32(picture.Pix[pixel+source])/255 - 0.5) / 0.5
			}
		}
	}
}
