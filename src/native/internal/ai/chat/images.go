package chat

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

// Limits on the pictures a message carries, as the desktop app and the backend enforce them.
const (
	ImageLimit        = 8
	ImageMaxBytes     = 2 << 20
	ImageMaxDimension = 2048
)

// Copy for the refusals of pictures. Each says what is allowed.
const (
	ImageLimitCopy     = "Attach no more than 8 images"
	ImageTotalCopy     = "AI chat images must total 2 MB or less"
	ImageSizeCopy      = "Each AI chat image must be 2 MB or smaller"
	ImageFormatCopy    = "AI chat images must be PNG, JPEG, WebP, or GIF"
	ImageDimensionCopy = "AI chat images must be valid and 2048 pixels or smaller"
)

// ErrImage is a picture the chat refuses; its message is the copy the palette shows.
var ErrImage = errors.New("chat: image refused")

// Image is a picture attached to a message: its name, its MIME type and its base64 data, as the backend takes it.
type Image struct {
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// NewImage reads data as a picture of a supported format within the size limits. The format comes from the bytes, not
// the name. WebP is checked for format and size only; its dimensions are left to the backend.
func NewImage(name string, data []byte) (Image, error) {
	mimeType := sniffMime(data)
	if mimeType == "" {
		return Image{}, refuse(ImageFormatCopy)
	}
	if len(data) > ImageMaxBytes {
		return Image{}, refuse(ImageSizeCopy)
	}
	if mimeType != "image/webp" {
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width > ImageMaxDimension || config.Height > ImageMaxDimension {
			return Image{}, refuse(ImageDimensionCopy)
		}
	}
	return Image{Name: strings.TrimSpace(name), MimeType: mimeType, Data: base64.StdEncoding.EncodeToString(data)}, nil
}

// Bytes returns the decoded size of the picture.
func (i Image) Bytes() int {
	return base64.StdEncoding.DecodedLen(len(i.Data))
}

// DataURL returns the picture as a data URL, the form the chat route takes it in.
func (i Image) DataURL() string {
	return "data:" + i.MimeType + ";base64," + i.Data
}

// AddImages returns attached with incoming appended, as many as the limits allow, and the copy of the limit that
// refused one, or "" when all were accepted.
func AddImages(attached, incoming []Image) ([]Image, string) {
	out := append([]Image(nil), attached...)
	total := 0
	for _, image := range out {
		total += image.Bytes()
	}
	problem := ""
	for _, image := range incoming {
		switch {
		case len(out) >= ImageLimit:
			return out, ImageLimitCopy
		case total+image.Bytes() > ImageTotalMaxBytes:
			problem = ImageTotalCopy
			continue
		}
		total += image.Bytes()
		out = append(out, image)
	}
	return out, problem
}

// ImageTotalMaxBytes is the decoded size the pictures of one message may total.
const ImageTotalMaxBytes = ImageMaxBytes

// sniffMime names the picture format of data from its leading bytes, or "" when it is none of the four.
func sniffMime(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case bytes.HasPrefix(data, []byte("\xff\xd8\xff")):
		return "image/jpeg"
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return "image/gif"
	case len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp"
	}
	return ""
}

// refuse wraps text as the refusal of a picture.
func refuse(text string) error {
	return &imageError{text: text}
}

// imageError is a refused picture; it matches ErrImage and reads as its copy.
type imageError struct{ text string }

func (e *imageError) Error() string { return e.text }

// Is makes a refused picture match ErrImage.
func (e *imageError) Is(target error) bool { return target == ErrImage }
