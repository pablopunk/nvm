package chat

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// MaxHistoryMessages bounds how many wire messages a request resends from the transcript.
const MaxHistoryMessages = 20

// maxMessageRunes bounds the text of one history message.
const maxMessageRunes = 12000

// ConversationSystemPrompt is the instruction every conversation starts with.
const ConversationSystemPrompt = "You are Nevermind AI, a helpful conversational assistant. Answer the user directly and concisely. " +
	"You cannot create, modify, validate, install, or remove Nevermind extensions. " +
	"You have tools for the user's files in their home folder, their installed apps, web links, and the clipboard. " +
	"Use a tool only when it helps the answer, and say what it found. Some tools wait for the user to allow them; " +
	"if the user refuses one, go on without it."

// unfinishedResult is what the model reads for a call whose result the transcript never recorded.
const unfinishedResult = "The tool did not finish, so there is no result."

// Request builds the chat messages for the next turn: the system prompt, then the most recent settled turns. Each
// assistant message that made tool calls is followed by one tool message per call, so the model reads what its tools
// returned. It skips notices, streaming replies and tool entries that did not finish; a stopped reply stays, so a
// resumed chat sees what the user saw. The caller makes sure the chat ends with the user message it is answering.
func Request(system string, chat Chat) []backend.Message {
	messages := []backend.Message{{Role: backend.RoleSystem, Content: backend.TextContent(system)}}
	results := finishedResults(chat.Messages)
	var turns []backend.Message
	picturesAt := latestPictureMessage(chat.Messages)
	for index, message := range chat.Messages {
		turns = append(turns, turnsOf(message, results, index == picturesAt)...)
	}
	if len(turns) > MaxHistoryMessages {
		turns = dropLeadingToolMessages(turns[len(turns)-MaxHistoryMessages:])
	}
	return append(messages, turns...)
}

// finishedResults maps the call id of each tool entry that has ended to the text the model reads for it.
func finishedResults(messages []Message) map[string]string {
	results := map[string]string{}
	for _, message := range messages {
		if message.Author == AuthorTool && message.Tool != nil && !toolStillRunning(message.Status) {
			results[message.Tool.CallID] = message.Text
		}
	}
	return results
}

// turnsOf converts one transcript entry into the wire messages it stands for. A tool entry is sent with the assistant
// message that called it, so it contributes nothing on its own. A user message's pictures go with it only when
// sendPictures is set, which the newest message that has pictures does; older ones keep a note that they were attached.
func turnsOf(message Message, results map[string]string, sendPictures bool) []backend.Message {
	settled := message.Status != StatusStreaming
	switch {
	case message.Author == AuthorUser && (message.Text != "" || len(message.Images) > 0):
		return []backend.Message{{Role: backend.RoleUser, Content: userContent(message, sendPictures)}}
	case message.Author == AuthorAssistant && settled && len(message.Calls) > 0:
		return callTurns(message, results)
	case message.Author == AuthorAssistant && settled && message.Text != "":
		return []backend.Message{{Role: backend.RoleAssistant, Content: backend.TextContent(truncateRunes(message.Text))}}
	}
	return nil
}

// callTurns is an assistant message with its calls, followed by the answer to each call. A call that never got an
// answer, such as one cut off by a restart, is answered as unfinished so the request stays valid.
func callTurns(message Message, results map[string]string) []backend.Message {
	assistant := backend.Message{Role: backend.RoleAssistant, ToolCalls: message.Calls}
	if message.Text != "" {
		assistant.Content = backend.TextContent(truncateRunes(message.Text))
	}
	turns := []backend.Message{assistant}
	for _, call := range message.Calls {
		text, ok := results[call.ID]
		if !ok {
			text = unfinishedResult
		}
		turns = append(turns, backend.Message{Role: backend.RoleTool, ToolCallID: call.ID, Content: backend.TextContent(truncateRunes(text))})
	}
	return turns
}

// dropLeadingToolMessages removes tool answers at the start of a window whose call was cut off.
func dropLeadingToolMessages(turns []backend.Message) []backend.Message {
	for len(turns) > 0 && turns[0].Role == backend.RoleTool {
		turns = turns[1:]
	}
	return turns
}

// truncateRunes keeps the first maxMessageRunes runes of text.
func truncateRunes(text string) string {
	if utf8.RuneCountInString(text) <= maxMessageRunes {
		return text
	}
	return string([]rune(text)[:maxMessageRunes])
}

// RequestSize is the number of characters a request carries, which the backend prices.
func RequestSize(messages []backend.Message) int {
	size := 0
	for _, message := range messages {
		size += len(message.Content)
	}
	return size
}

// toolStillRunning reports whether a tool entry has not ended yet.
func toolStillRunning(status Status) bool {
	return status == StatusRunning || status == StatusAwaiting
}

// latestPictureMessage is the index of the newest user message that carries pictures, or -1 when there is none.
func latestPictureMessage(messages []Message) int {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Author == AuthorUser && len(messages[index].Images) > 0 {
			return index
		}
	}
	return -1
}

// userContent is the content of a user message: its text alone, or with its pictures as parts when sendPictures is set.
// A message of pictures alone is asked about them, as the desktop app asks.
func userContent(message Message, sendPictures bool) json.RawMessage {
	text := truncateRunes(message.Text)
	if len(message.Images) == 0 {
		return backend.TextContent(text)
	}
	if text == "" {
		text = picturePrompt(len(message.Images))
	}
	if !sendPictures {
		return backend.TextContent(text + "\n" + pictureNote(len(message.Images)))
	}
	parts := []backend.ContentPart{backend.TextPart(text)}
	for _, image := range message.Images {
		parts = append(parts, backend.ImagePart(image.DataURL()))
	}
	return backend.PartsContent(parts)
}

// picturePrompt asks about pictures sent with no words, in the desktop app's words.
func picturePrompt(count int) string {
	if count == 1 {
		return "Analyze the attached image."
	}
	return "Analyze the attached images."
}

// pictureNote says in the transcript that pictures were attached, for a message the model no longer sees them in.
func pictureNote(count int) string {
	if count == 1 {
		return "[Attached image]"
	}
	return fmt.Sprintf("[Attached %d images]", count)
}
