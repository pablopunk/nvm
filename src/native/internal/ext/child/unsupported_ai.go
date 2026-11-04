package child

import (
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// unsupported is the error an accessor returns when this host does not serve it yet. It wraps api.ErrNotAvailable,
// so extension code classifies it like any unavailable service, and it names the service for the palette.
func unsupported(service string) error {
	return fmt.Errorf("%s is not available in this Nevermind version yet: %w", service, api.ErrNotAvailable)
}

// unsupportedAI answers every model call with the unsupported error. The AI service is not served by the host yet.
type unsupportedAI struct{}

func (unsupportedAI) IsAvailable() (bool, error) { return false, unsupported("ctx.ai") }

func (unsupportedAI) Decide(api.DecisionRequest) (api.DecisionResult, error) {
	return api.DecisionResult{}, unsupported("ctx.ai.decide")
}

func (unsupportedAI) Ask(string, api.AIOptions) (string, error) {
	return "", unsupported("ctx.ai.ask")
}

func (unsupportedAI) Stream(string, api.AIOptions) (api.AIStream, error) {
	return nil, unsupported("ctx.ai.stream")
}

func (unsupportedAI) Prepare(api.AIOptions) error { return unsupported("ctx.ai.prepare") }

func (unsupportedAI) Attachments() api.AttachmentBuilder { return attachmentBuilder{} }

func (unsupportedAI) Session(string, api.AIOptions) (api.AISession, error) {
	return nil, unsupported("ctx.ai.session")
}

// attachmentBuilder builds the attachments of a prompt as data. Reading the selection, the clipboard or OCR needs the
// host, so those fail with the unsupported error.
type attachmentBuilder struct{}

func (attachmentBuilder) Text(text, title string) api.Attachment {
	return api.Attachment{Kind: "text", Text: text, Title: title}
}

func (attachmentBuilder) Image(pathOrDataURL, title string, ocr bool) api.Attachment {
	return api.Attachment{Kind: "image", Path: pathOrDataURL, Title: title, OCR: ocr}
}

func (attachmentBuilder) File(path, as, title string) api.Attachment {
	return api.Attachment{Kind: "file", Path: path, As: as, Title: title}
}

func (attachmentBuilder) SelectedText(string) (api.Attachment, error) {
	return api.Attachment{}, unsupported("ctx.ai.attachments.selectedText")
}

func (attachmentBuilder) SelectedFiles(string) ([]api.Attachment, error) {
	return nil, unsupported("ctx.ai.attachments.selectedFiles")
}

func (attachmentBuilder) Clipboard(string) ([]api.Attachment, error) {
	return nil, unsupported("ctx.ai.attachments.clipboard")
}

func (attachmentBuilder) OCRImage(string, string) (api.Attachment, error) {
	return api.Attachment{}, unsupported("ctx.ai.attachments.ocrImage")
}

// unsupportedDictation answers every dictation call with the unsupported error.
type unsupportedDictation struct{}

func (unsupportedDictation) Status() (api.DictationStatus, error) {
	return "", unsupported("ctx.dictation")
}

func (unsupportedDictation) APIAvailable() (bool, error) { return false, unsupported("ctx.dictation") }

func (unsupportedDictation) Devices() ([]api.DictationDevice, error) {
	return nil, unsupported("ctx.dictation.devices")
}

func (unsupportedDictation) Start(api.DictationStartOptions) error {
	return unsupported("ctx.dictation.start")
}

func (unsupportedDictation) Stop() (string, error) { return "", unsupported("ctx.dictation.stop") }

func (unsupportedDictation) Cancel() error { return unsupported("ctx.dictation.cancel") }

// unsupportedAIBuilder answers builder calls. Only built-in extensions may use the builder, so an extension gets an
// action the palette shows as unavailable and no chat data.
type unsupportedAIBuilder struct{}

func (unsupportedAIBuilder) StartChat(_, title string) api.Action { return unsupportedAction(title) }

func (unsupportedAIBuilder) OpenChat(string) api.Action { return unsupportedAction("Open chat") }

func (unsupportedAIBuilder) RemoveChat(string) api.Action { return unsupportedAction("Remove chat") }

func (unsupportedAIBuilder) TweakExtension(api.TweakRequest) api.Action {
	return unsupportedAction("Tweak extension")
}

func (unsupportedAIBuilder) OpenChatsList(title string) api.Action { return unsupportedAction(title) }

func (unsupportedAIBuilder) ListChats() []api.ChatSummary { return nil }

func (unsupportedAIBuilder) CleanupExpiredConversations() (int, error) {
	return 0, unsupported("ctx.aiBuilder.cleanupExpiredConversations")
}

// unsupportedAction is a window-free placeholder the host reports as unavailable.
func unsupportedAction(title string) api.Action {
	return api.Action{Kind: "unsupported", Title: title}
}

// unsupportedOwnership says that no builder conversation owns any file, which is true for every extension but the
// built-ins.
type unsupportedOwnership struct{}

func (unsupportedOwnership) OwnerOf(string) (string, bool) { return "", false }

func (unsupportedOwnership) FilesForChat(string) []string { return nil }

func (unsupportedOwnership) CanWrite(string, string) bool { return false }

// unsupportedUpdates reports that the update service is not served, so the palette does not offer updates.
type unsupportedUpdates struct{}

func (unsupportedUpdates) State() api.UpdateState {
	return api.UpdateState{Status: "unavailable", ErrorMessage: "Updates are not available in this Nevermind version yet."}
}
