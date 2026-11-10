package app

import (
	"context"
	"log"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// A builder chat and its extension are one thing: deleting either deletes the other. Each direction ends the other
// without looping, because by then the thing that started it is already gone.

// removeExtensionOfChat deletes the extension of a deleted builder chat. An extension that another chat still owns stays,
// which only older data can have. The removal runs off the UI thread because it stops the extension's process.
func (p *App) removeExtensionOfChat(record chat.Chat) {
	if record.Kind != chat.KindBuilder || record.Extension == "" || len(p.chats.Owners(record.Extension)) > 0 {
		return
	}
	extension := record.Extension
	safego.Go("chat.remove-extension", func(context.Context) {
		ctl := p.currentExtensionController()
		if ctl == nil {
			return
		}
		if err := ctl.Uninstall(extension); err != nil {
			log.Println("chat: could not remove the extension of a deleted chat:", err)
		}
	})
}

// removeChatsOfExtension deletes the builder chats of a removed extension.
func (p *App) removeChatsOfExtension(extension string) {
	p.onUI(func() {
		if p.chats == nil {
			return
		}
		for _, owner := range p.chats.Owners(extension) {
			if err := p.chats.Delete(owner.ID); err != nil {
				log.Println("chat: could not delete the chat of a removed extension:", err)
			}
		}
	})
}
