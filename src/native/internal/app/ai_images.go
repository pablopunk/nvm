package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// chatImageChooser is the operating system's chooser for pictures a chat message may carry.
var chatImageChooser = platform.ChooseRequest{Title: "Attach Images", ButtonLabel: "Attach", Extensions: []string{"png", "jpg", "jpeg", "webp", "gif"}, FilterName: "Images", Multiple: true}

// unreadablePicture is the line shown for a picture file that cannot be read.
const unreadablePicture = "That picture could not be read."

// chooseChatImages opens the operating system's chooser for pictures and hands the ones read to done, on the UI thread.
// The chooser blocks, so it runs off the UI thread; a canceled chooser hands over nothing.
func (p *App) chooseChatImages(done func([]chat.Image)) {
	dialog := p.ports.FileDialog
	if dialog == nil {
		p.Toast(chooserUnavailable)
		return
	}
	safego.Go("ai-chat.attach", func(context.Context) {
		paths, err := dialog.Choose(chatImageChooser)
		images, problem := readChatImages(paths)
		p.onUI(func() {
			if err != nil {
				p.Toast(chooserUnavailable)
				return
			}
			done(images)
			if problem != "" {
				p.Toast(problem)
			}
		})
	})
}

// readChatImages reads each picture file into a chat image. It returns the pictures read and the copy of the first
// problem, or "" when every file was read.
func readChatImages(paths []string) ([]chat.Image, string) {
	var images []chat.Image
	problem := ""
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			problem = unreadablePicture
			continue
		}
		image, err := chat.NewImage(filepath.Base(path), data)
		if err != nil {
			problem = err.Error()
			continue
		}
		images = append(images, image)
	}
	return images, problem
}

// clipboardChatImage reads the picture on the clipboard. ok is false when the clipboard holds no picture; err says why a
// picture there was refused.
func (p *App) clipboardChatImage() (chat.Image, bool, error) {
	if p.ports.Clipboard == nil {
		return chat.Image{}, false, nil
	}
	data, err := p.ports.Clipboard.ReadImagePNG()
	if errors.Is(err, platform.ErrNotFound) {
		return chat.Image{}, false, nil
	}
	if err != nil {
		return chat.Image{}, false, err
	}
	image, err := chat.NewImage("", data)
	if err != nil {
		return chat.Image{}, false, err
	}
	return image, true, nil
}
