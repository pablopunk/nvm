package aichat

import "github.com/pablopunk/nvm/native/internal/ai/chat"

// ContinueLabels are the root row's title and subtitle for a chat of kind named title. A builder chat says so, and a
// conversation says so, as the desktop app's Continue rows do.
func ContinueLabels(kind chat.Kind, title string) (string, string) {
	if kind == chat.KindBuilder {
		return "Continue AI chat: " + title, kindLabel(kind)
	}
	return "Continue conversation: " + title, kindLabel(kind)
}

// ChatIcon is the icon that marks a chat of kind in the lists.
func ChatIcon(kind chat.Kind) string {
	if kind == chat.KindBuilder {
		return "sparkles"
	}
	return "message-circle"
}

// kindLabel names a chat of kind in a subtitle.
func kindLabel(kind chat.Kind) string {
	if kind == chat.KindBuilder {
		return "AI builder chat"
	}
	return "AI conversation"
}
