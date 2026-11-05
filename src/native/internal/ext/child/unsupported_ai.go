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

// unsupportedAction is a placeholder row the palette reports as unavailable when it runs. It validates, so the view
// that carries it still loads.
func unsupportedAction(title string) api.Action {
	return api.Action{Kind: api.ActionUnavailable, Title: title}
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
