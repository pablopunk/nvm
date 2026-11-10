package thumbs

import (
	"bytes"
	"encoding/binary"
	"image"
)

// jpegOrientation reads EXIF orientation only within declared JPEG segments and before the image scan.
func jpegOrientation(data []byte) int {
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}
	for pos := 2; pos < len(data); {
		if data[pos] != 0xff {
			return 1
		}
		for pos < len(data) && data[pos] == 0xff {
			pos++
		}
		if pos == len(data) {
			return 1
		}
		marker := data[pos]
		pos++
		if marker == 0xda || marker == 0xd9 {
			return 1
		}
		if marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			continue
		}
		if marker == 0x00 || marker == 0xd8 || len(data)-pos < 2 {
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		if size < 2 || size > len(data)-pos {
			return 1
		}
		end := pos + size
		if marker == 0xe1 && size >= 8 && bytes.Equal(data[pos+2:pos+8], []byte("Exif\x00\x00")) {
			return tiffOrientation(data[pos+8 : end])
		}
		pos = end
	}
	return 1
}

func tiffOrientation(data []byte) int {
	if len(data) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(data[2:4]) != 42 {
		return 1
	}
	offset := uint64(order.Uint32(data[4:8]))
	if offset < 8 || offset > uint64(len(data)-2) {
		return 1
	}
	pos := int(offset)
	count := int(order.Uint16(data[pos : pos+2]))
	pos += 2
	for i := 0; i < count; i++ {
		if len(data)-pos < 12 {
			return 1
		}
		entry := data[pos : pos+12]
		if order.Uint16(entry[:2]) == 0x0112 {
			if order.Uint16(entry[2:4]) != 3 || order.Uint32(entry[4:8]) != 1 {
				return 1
			}
			value := int(order.Uint16(entry[8:10]))
			if value >= 1 && value <= 8 {
				return value
			}
			return 1
		}
		pos += 12
	}
	return 1
}

// orientThumbnail applies all eight EXIF transforms after scaling, keeping the extra allocation thumbnail-sized.
func orientThumbnail(src *image.NRGBA, orientation int) *image.NRGBA {
	if orientation < 2 || orientation > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	out := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch orientation {
			case 2:
				dx = w - 1 - x
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dy = h - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			out.SetNRGBA(dx, dy, src.NRGBAAt(x, y))
		}
	}
	return out
}
