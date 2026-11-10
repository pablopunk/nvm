package thumbs

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestRenderPreservesJPEGOrientation(t *testing.T) {
	for _, tc := range []struct {
		name          string
		orientation   uint16
		order         binary.ByteOrder
		width, height int
		firstBlue     bool
		xmpFirst      bool
	}{
		{"rotate-clockwise", 6, binary.LittleEndian, 30, 60, false, false},
		{"xmp-first-rotate", 6, binary.LittleEndian, 30, 60, false, true},
		{"mirror", 2, binary.BigEndian, 60, 30, true, false},
		{"xmp-first-mirror", 2, binary.BigEndian, 60, 30, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, 60, 30))
			for y := 0; y < 30; y++ {
				for x := 0; x < 60; x++ {
					col := color.NRGBA{R: 240, A: 255}
					if x >= 30 {
						col = color.NRGBA{B: 240, A: 255}
					}
					img.SetNRGBA(x, y, col)
				}
			}
			var jpg bytes.Buffer
			if err := jpeg.Encode(&jpg, img, &jpeg.Options{Quality: 100}); err != nil {
				t.Fatal(err)
			}
			tiff := make([]byte, 26)
			if tc.order == binary.LittleEndian {
				copy(tiff, "II")
			} else {
				copy(tiff, "MM")
			}
			tc.order.PutUint16(tiff[2:], 42)
			tc.order.PutUint32(tiff[4:], 8)
			tc.order.PutUint16(tiff[8:], 1)
			tc.order.PutUint16(tiff[10:], 0x0112)
			tc.order.PutUint16(tiff[12:], 3)
			tc.order.PutUint32(tiff[14:], 1)
			tc.order.PutUint16(tiff[18:], tc.orientation)
			exif := append([]byte("Exif\x00\x00"), tiff...)
			data := append([]byte{0xff, 0xd8, 0xff, 0xe1, byte((len(exif) + 2) >> 8), byte(len(exif) + 2)}, exif...)
			data = append(data, jpg.Bytes()[2:]...)
			if tc.xmpFirst {
				xmp := []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta/>")
				prefix := append([]byte{0xff, 0xd8, 0xff, 0xe1, byte((len(xmp) + 2) >> 8), byte(len(xmp) + 2)}, xmp...)
				data = append(prefix, data[2:]...)
			}
			fsys := fake.NewFS()
			fsys.Seed("/photo.jpg", data)
			out, err := Render(fsys, "/photo.jpg", 128)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := png.Decode(bytes.NewReader(out))
			if err != nil {
				t.Fatal(err)
			}
			if got := decoded.Bounds().Size(); got != image.Pt(tc.width, tc.height) {
				t.Fatalf("size %v, want %dx%d", got, tc.width, tc.height)
			}
			r, _, b, _ := decoded.At(5, 5).RGBA()
			if (b > r) != tc.firstBlue {
				t.Fatalf("wrong top-left orientation: red=%d blue=%d", r, b)
			}
		})
	}
}

func TestJPEGOrientationRejectsMalformedSegments(t *testing.T) {
	cases := [][]byte{
		nil, {0xff, 0xd8, 0xff}, {0xff, 0xd8, 0xff, 0xe1, 0x00},
		{0xff, 0xd8, 0xff, 0xe1, 0x00, 0x01}, {0xff, 0xd8, 0xff, 0xe1, 0xff, 0xff, 0x45, 0x78},
		{0xff, 0xd8, 0xff, 0xe1, 0x00, 0x06, 'X', 'M', 'P', 0, 0xff, 0xe1, 0x00, 0x08, 'E', 'x', 'i', 'f', 0},
		{0xff, 0xd8, 0xff, 0xda, 0x00, 0x02, 0xff, 0xe1, 0x00, 0x08, 'E', 'x', 'i', 'f', 0, 0},
	}
	for _, input := range cases {
		if got := jpegOrientation(input); got != 1 {
			t.Fatalf("malformed %x orientation = %d", input, got)
		}
	}
}

func TestOrientThumbnailAllEXIFTransforms(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(y*2 + x + 1), A: 255})
		}
	}
	expected := [][]uint8{{1, 2, 3, 4, 5, 6}, {2, 1, 4, 3, 6, 5}, {6, 5, 4, 3, 2, 1}, {5, 6, 3, 4, 1, 2}, {1, 3, 5, 2, 4, 6}, {5, 3, 1, 6, 4, 2}, {6, 4, 2, 5, 3, 1}, {2, 4, 6, 1, 3, 5}}
	for i, want := range expected {
		out := orientThumbnail(src, i+1)
		var got []uint8
		for y := 0; y < out.Bounds().Dy(); y++ {
			for x := 0; x < out.Bounds().Dx(); x++ {
				got = append(got, out.NRGBAAt(x, y).R)
			}
		}
		if !bytes.Equal(got, want) {
			t.Errorf("orientation %d: got %v want %v", i+1, got, want)
		}
	}
}
