package chat

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// pngBytes encodes a w by h picture as PNG.
func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, 0, color.RGBA{R: 200, A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// TestNewImageAcceptsAPictureAndNamesItsFormat checks a PNG is accepted with its type read from its bytes.
func TestNewImageAcceptsAPictureAndNamesItsFormat(t *testing.T) {
	img, err := NewImage("shot.png", pngBytes(t, 8, 6))
	if err != nil {
		t.Fatalf("NewImage: %v", err)
	}
	if img.MimeType != "image/png" || img.Name != "shot.png" || !strings.HasPrefix(img.DataURL(), "data:image/png;base64,") {
		t.Fatalf("image = %+v, want a PNG data URL named shot.png", img)
	}
}

// TestNewImageRefusesWhatTheDesktopAppRefuses checks text, an oversized picture and one past 2048 pixels are refused
// with the desktop app's copy.
func TestNewImageRefusesWhatTheDesktopAppRefuses(t *testing.T) {
	if _, err := NewImage("note.txt", []byte("hello")); !errors.Is(err, ErrImage) || err.Error() != ImageFormatCopy {
		t.Fatalf("text: err = %v, want the format copy", err)
	}
	big := append(pngBytes(t, 8, 8), make([]byte, ImageMaxBytes)...)
	if _, err := NewImage("big.png", big); err == nil || err.Error() != ImageSizeCopy {
		t.Fatalf("oversized: err = %v, want the size copy", err)
	}
	if _, err := NewImage("wide.png", pngBytes(t, ImageMaxDimension+1, 2)); err == nil || err.Error() != ImageDimensionCopy {
		t.Fatalf("wide: err = %v, want the dimension copy", err)
	}
}

// TestAddImagesStopsAtEightPictures checks the ninth picture is refused with the limit's copy and the eight kept.
func TestAddImagesStopsAtEightPictures(t *testing.T) {
	img := Image{MimeType: "image/png", Data: "AAAA"}
	incoming := make([]Image, ImageLimit+1)
	for i := range incoming {
		incoming[i] = img
	}
	kept, problem := AddImages(nil, incoming)
	if len(kept) != ImageLimit || problem != ImageLimitCopy {
		t.Fatalf("kept %d, problem %q; want %d and the limit copy", len(kept), problem, ImageLimit)
	}
}

// TestRequestSendsTheNewestPicturesAsParts checks the newest picture message goes to the model as image parts, and an
// older one keeps a note that its pictures were attached.
func TestRequestSendsTheNewestPicturesAsParts(t *testing.T) {
	picture := Image{MimeType: "image/png", Data: "AAAA"}
	chat := Chat{Messages: []Message{
		{Author: AuthorUser, Text: "first", Images: []Image{picture}, Status: StatusComplete},
		{Author: AuthorAssistant, Text: "seen", Status: StatusComplete},
		{Author: AuthorUser, Images: []Image{picture, picture}, Status: StatusComplete},
	}}
	turns := Request("system", chat)[1:]

	var older string
	if err := json.Unmarshal(turns[0].Content, &older); err != nil || older != "first\n[Attached image]" {
		t.Fatalf("older message = %q (%v), want its text and a note", older, err)
	}
	var parts []backend.ContentPart
	if err := json.Unmarshal(turns[2].Content, &parts); err != nil {
		t.Fatalf("newest content is not a part list: %v", err)
	}
	if len(parts) != 3 || parts[0].Text != "Analyze the attached images." || parts[1].ImageURL == nil ||
		parts[1].ImageURL.URL != "data:image/png;base64,AAAA" {
		t.Fatalf("newest parts = %+v, want the prompt and two data URLs", parts)
	}
}
