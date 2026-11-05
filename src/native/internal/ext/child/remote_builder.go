package child

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteBuilder is the AI builder of a built-in extension. Reads go to the host; the actions describe what the user
// picks, and the palette runs them.
type remoteBuilder struct {
	link *hostLink
}

func (b remoteBuilder) StartChat(prompt, title string) api.Action {
	return builderAction(firstNonEmpty(title, "Automate"), api.BuilderStart, api.BuilderAction{Prompt: prompt})
}

func (b remoteBuilder) OpenChat(chatID string) api.Action {
	return builderAction("Open Chat", api.BuilderOpen, api.BuilderAction{ChatID: chatID})
}

func (b remoteBuilder) RemoveChat(chatID string) api.Action {
	action := builderAction("Remove Chat", api.BuilderRemove, api.BuilderAction{ChatID: chatID})
	action.Confirm = &api.Confirmation{Title: "Remove this chat?", ConfirmLabel: "Remove", Destructive: true}
	return action
}

func (b remoteBuilder) TweakExtension(request api.TweakRequest) api.Action {
	return builderAction(firstNonEmpty(request.Title, "Tweak with AI"), api.BuilderTweak, api.BuilderAction{ExtensionID: request.ExtensionID, Prompt: request.Prompt})
}

func (b remoteBuilder) OpenChatsList(title string) api.Action {
	return builderAction(firstNonEmpty(title, "AI Chats"), api.BuilderList, api.BuilderAction{})
}

func (b remoteBuilder) ListChats() []api.ChatSummary {
	var chats []api.ChatSummary
	_ = b.link.Call(protocol.ServiceBuilder, protocol.OpBuilderList, nil, &chats)
	return chats
}

func (b remoteBuilder) GetChat(chatID string) (api.ChatDetail, bool) {
	var found protocol.AIChatFound
	if err := b.link.Call(protocol.ServiceBuilder, protocol.OpBuilderGet, protocol.AIChatGetArgs{ID: chatID}, &found); err != nil {
		return api.ChatDetail{}, false
	}
	return found.Chat, found.Found
}

func (b remoteBuilder) CleanupExpiredConversations() (int, error) {
	var answer protocol.BuilderCleanupAnswer
	err := b.link.Call(protocol.ServiceBuilder, protocol.OpBuilderCleanup, nil, &answer)
	return answer.Removed, err
}

// ExtensionPreview asks the host for the preview view of an installed extension.
func (b remoteBuilder) ExtensionPreview(extensionID string) (api.View, bool) {
	var answer protocol.BuilderPreviewAnswer
	if err := b.link.Call(protocol.ServiceBuilder, protocol.OpExtensionPreview, protocol.BuilderExtensionArgs{Extension: extensionID}, &answer); err != nil {
		return api.View{}, false
	}
	return answer.View, answer.Found
}

// remoteOwnership says which builder chat wrote an extension and whether a chat may change it.
type remoteOwnership struct {
	link *hostLink
}

func (o remoteOwnership) OwnerOf(extensionFile string) (string, bool) {
	var answer protocol.BuilderOwnerAnswer
	if err := o.link.Call(protocol.ServiceBuilder, protocol.OpOwnerOf, protocol.BuilderExtensionArgs{Extension: extensionFile}, &answer); err != nil {
		return "", false
	}
	return answer.ChatID, answer.Found
}

func (o remoteOwnership) FilesForChat(chatID string) []string {
	var files []string
	_ = o.link.Call(protocol.ServiceBuilder, protocol.OpFilesForChat, protocol.BuilderFilesArgs{ChatID: chatID}, &files)
	return files
}

func (o remoteOwnership) CanWrite(extensionFile, chatID string) bool {
	var allowed bool
	_ = o.link.Call(protocol.ServiceBuilder, protocol.OpCanWrite, protocol.BuilderCanWriteArgs{Extension: extensionFile, ChatID: chatID}, &allowed)
	return allowed
}

// builderAction is a declarative builder action with its payload.
func builderAction(title, op string, payload api.BuilderAction) api.Action {
	return api.Action{Kind: api.ActionBuilder, Op: op, Title: title, Builder: &payload}
}

// firstNonEmpty returns the first of the values that is not empty.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
