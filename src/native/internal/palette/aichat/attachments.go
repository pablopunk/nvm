package aichat

import (
	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design"
)

// attachmentSize is the side of a picture's thumbnail in the composer, in DIPs.
const attachmentSize = 48

// attachButton opens the chooser for pictures, and done receives the ones read. The AI Chats page offers it on the Smart
// tier only, as the desktop app does.
func attachButton(c *ui.Context, pick func(done func([]chat.Image)), done func([]chat.Image)) {
	k := design.Tokens(c)
	button := ui.ButtonBase(c).Padding(3, 6).Radius(6).Label("Attach images")
	if button.Hovered() {
		button.Background(k.Interactive)
	}
	button.Children(func() {
		ui.Icon(c, design.Icon("image")).FontSize(14).TextColor(k.Secondary)
	})
	if button.Clicked() {
		pick(done)
	}
}

// attachmentsRow is the row of thumbnails the next message carries, each with a control that removes it by index.
func attachmentsRow(c *ui.Context, images []chat.Image, removeAt func(index int)) {
	k := design.Tokens(c)
	removed := -1
	ui.Row(c).Gap(8).Padding(8, 8, 0, 8).AlignItems(ui.Center).Children(func() {
		for index, image := range images {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				width, height := fittedPicture(image, attachmentSize, attachmentSize)
				picture(c, image, width, height)
				remove := ui.ButtonBase(c).Padding(3).Radius(10).Label("Remove " + imageLabel(image, index))
				if remove.Hovered() {
					remove.Background(k.Interactive)
				}
				remove.Children(func() {
					ui.Icon(c, design.Icon("x")).FontSize(11).TextColor(k.Muted)
				})
				if remove.Clicked() {
					removed = index
				}
			})
		}
	})
	if removed >= 0 {
		removeAt(removed)
	}
}

// imageLabel names a picture for the controls that act on it: its file name, or its place in the row.
func imageLabel(image chat.Image, index int) string {
	if image.Name != "" {
		return image.Name
	}
	return "image " + string(rune('1'+index))
}
