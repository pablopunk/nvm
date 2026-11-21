package fixtures

import (
	"fmt"
	"time"
)

// chatEpoch is the last-updated time of the first chat; each later chat is an hour older.
var chatEpoch = time.Date(2026, time.February, 1, 18, 0, 0, 0, time.UTC)

// chatTitles are the titles of generated chats, reused with a number past the end of the list.
var chatTitles = []string{
	"Plan a weekend in Lisbon", "Explain goroutine leaks", "Rewrite the release notes",
	"Summarize the budget memo", "Naming a new feature flag", "Debug a flaky test",
}

// Message is one turn of a chat.
type Message struct {
	Role    string
	Content string
}

// Chat is a conversation as the chat history stores it.
type Chat struct {
	ID       string
	Title    string
	Updated  time.Time
	Messages []Message
}

// Chats returns n chats, most recently updated first. Each has a user question and an assistant answer,
// which is the content the fake backend streams back for the same question.
func Chats(n int) []Chat {
	chats := make([]Chat, 0, max(n, 0))
	for i := range n {
		title := chatTitles[i%len(chatTitles)]
		if i >= len(chatTitles) {
			title = fmt.Sprintf("%s (%d)", title, i/len(chatTitles)+1)
		}
		question := "Question " + fmt.Sprint(i+1) + ": " + title
		chats = append(chats, Chat{
			ID:      fmt.Sprintf("chat-%03d", i+1),
			Title:   title,
			Updated: chatEpoch.Add(-time.Duration(i) * time.Hour),
			Messages: []Message{
				{Role: "user", Content: question},
				{Role: "assistant", Content: FixtureReply},
			},
		})
	}
	return chats
}
