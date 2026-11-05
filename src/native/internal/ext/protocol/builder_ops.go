package protocol

// Operations of the builder service, which only built-in extensions may call. The list and get reads return the
// builder chats; the ownership reads say which chat wrote or may change an extension.
const (
	OpBuilderList    = "list"
	OpBuilderGet     = "get"
	OpBuilderCleanup = "cleanup"
	OpOwnerOf        = "owner-of"
	OpFilesForChat   = "files-for-chat"
	OpCanWrite       = "can-write"
)

// BuilderCleanupAnswer is the answer to cleanup: how many expired conversations were removed.
type BuilderCleanupAnswer struct {
	Removed int `json:"removed"`
}

// BuilderExtensionArgs names an extension; ChatID names a chat.
type BuilderExtensionArgs struct {
	Extension string `json:"extension"`
}

// BuilderFilesArgs names a chat whose extensions are listed.
type BuilderFilesArgs struct {
	ChatID string `json:"chatId"`
}

// BuilderCanWriteArgs asks whether a chat may change an extension.
type BuilderCanWriteArgs struct {
	Extension string `json:"extension"`
	ChatID    string `json:"chatId"`
}

// BuilderOwnerAnswer is the answer to owner-of: the chat that owns the extension, when Found is set.
type BuilderOwnerAnswer struct {
	ChatID string `json:"chatId"`
	Found  bool   `json:"found"`
}
