package aichat

import (
	"encoding/base64"
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
)

// thumbnailLimit bounds how many decoded pictures stay in memory; the cache starts again past it.
const thumbnailLimit = 32

// thumbnails keeps the decoded pictures the chat draws, keyed by their data, so a frame does not decode them again.
// Only the UI thread draws, so it needs no lock.
var thumbnails = map[string]ui.ImageSource{}

// thumbnail returns the decoded picture of image, or nil when its bytes do not decode.
func thumbnail(image chat.Image) ui.ImageSource {
	key := thumbnailKey(image.Data)
	if src, ok := thumbnails[key]; ok {
		return src
	}
	if len(thumbnails) >= thumbnailLimit {
		thumbnails = map[string]ui.ImageSource{}
	}
	var src ui.ImageSource
	if data, err := base64.StdEncoding.DecodeString(image.Data); err == nil {
		if bitmap, err := ui.DecodeBitmap(data); err == nil {
			src = bitmap
		}
	}
	thumbnails[key] = src
	return src
}

// thumbnailKey names a picture's data by its length and its ends, which tell pictures apart without hashing them.
func thumbnailKey(data string) string {
	if len(data) <= 128 {
		return data
	}
	return fmt.Sprintf("%d:%s:%s", len(data), data[:64], data[len(data)-64:])
}

// picture draws image at width by height, or a placeholder where it does not decode.
func picture(c *ui.Context, image chat.Image, width, height float32) {
	k := design.Tokens(c)
	src := thumbnail(image)
	if src == nil {
		ui.Column(c).Width(width).Height(height).Radius(6).Background(k.Interactive).Border(1, k.EdgeSubtle).Center().Children(func() {
			ui.Icon(c, design.Icon("image")).FontSize(14).TextColor(k.Muted)
		})
		return
	}
	ui.Image(c, src).Width(width).Height(height).Radius(6)
}

// fittedPicture is the size a picture of its own shape takes when it must fit a box of maxWidth by maxHeight.
func fittedPicture(image chat.Image, maxWidth, maxHeight float32) (float32, float32) {
	src := thumbnail(image)
	if src == nil {
		return maxWidth / 2, maxHeight / 2
	}
	w, h := ui.ImageSize(src)
	if w <= 0 || h <= 0 {
		return maxWidth / 2, maxHeight / 2
	}
	scale := min(1, maxWidth/w, maxHeight/h)
	return w * scale, h * scale
}

// messagePictures draws the pictures of a user message, right-aligned above its text as the desktop app's are.
func messagePictures(c *ui.Context, images []chat.Image) {
	ui.Row(c).Gap(6).Children(func() {
		ui.Spacer(c)
		ui.Row(c).Gap(6).Wrap().MaxWidth(440).Children(func() {
			for _, image := range images {
				width, height := fittedPicture(image, 200, 160)
				picture(c, image, width, height)
			}
		})
	})
}
