package builder

import (
	"context"
	"errors"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// ErrExtensionsLoading is the answer while the extension manager has not started. The builder reports it and does not
// ask the model again, since the model cannot fix it.
var ErrExtensionsLoading = errors.New("extensions are still loading")

// Conversation is what the builder reads and writes on the chat service: the chats, the owners of an extension, the
// step a chat shows while a write runs, and the extension a chat is bound to. chat.Service satisfies it.
type Conversation interface {
	Chat(id string) (chat.Chat, bool)
	ReserveExtension(id, target string, turn *chat.BuilderTurn) error
	Owners(extension string) []chat.Chat
	Bind(id, extension string) error
	SetStep(id, label string)
	ClearStep(id string)
}

// Extensions is what the builder reads and writes on the extension manager. Check loads candidate files in a child
// process without publishing them and returns the protocol methods the source implements. EditSnapshot returns
// a bounded source snapshot or confirmed absence with an opaque revision. InstallEdit conditionally publishes it.
type Extensions interface {
	Definition(id string) (api.Manifest, string, error)
	Check(id string, files map[string][]byte) ([]string, error)
	EditSnapshot(id string) (map[string][]byte, any, error)
	InstallEdit(ctx context.Context, id string, files map[string][]byte, enable bool, revision any) error
	// Catalog lists the installed extensions, so the builder can see what exists before it writes.
	Catalog() []Installed
}
