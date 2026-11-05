package aichats

// Scores, limits and timings. The scores order the rows as the desktop app's root does.
const (
	rootChatLimit         = 4
	scoreRootConversation = 13
	scoreAutomate         = 40
	scoreAIChatsCommand   = 16
	oneLineLimit          = 90
	streamTickMs          = 150
	cleanupStartupDelayMs = 1000
	kindBuilder           = "builder"
)

// continueLabels are the root row's title and subtitle for a chat of kind named title. A builder chat says so, and a
// conversation says so, as the desktop app's Continue rows do.
func continueLabels(kind, title string) (string, string) {
	if kind == "builder" {
		return "Continue AI chat: " + title, kindLabel(kind)
	}
	return "Continue conversation: " + title, kindLabel(kind)
}

// chatTiePriority puts a builder chat after the other rows of equal score and last use, as 1.x does.
func chatTiePriority(kind string) int {
	if kind == kindBuilder {
		return -1
	}
	return 0
}

// chatIcon is the icon that marks a chat of kind in the lists.
func chatIcon(kind string) string {
	if kind == "builder" {
		return "sparkles"
	}
	return "message-circle"
}

// kindLabel names a chat of kind in a subtitle.
func kindLabel(kind string) string {
	if kind == "builder" {
		return "AI builder chat"
	}
	return "AI conversation"
}
