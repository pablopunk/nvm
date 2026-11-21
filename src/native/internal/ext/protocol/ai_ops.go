package protocol

import "github.com/pablopunk/nvm/native/internal/ext/api"

// Operations of the ai service. Ask, decide and stream-open take an optional session. A stream is pulled: stream-open starts it on the host, and stream-next returns
// the events that arrived since the last pull, waiting for the next one when none has. The host answers each pull
// as a service call, so the wait is host time and does not count against the extension's budget.
const (
	OpIsAvailable     = "is-available"
	OpPrepare         = "prepare"
	OpAsk             = "ask"
	OpDecide          = "decide"
	OpStreamOpen      = "stream-open"
	OpStreamNext      = "stream-next"
	OpStreamAbort     = "stream-abort"
	OpSessionReset    = "session-reset"
	OpAttachClipboard = "attach-clipboard"
	OpAttachOCR       = "attach-ocr"
	OpChatList        = "chat-list"
	OpChatGet         = "chat-get"
)

// AIChatGetArgs names the chat to read.
type AIChatGetArgs struct {
	ID string `json:"id"`
}

// AIChatFound is the answer to chat-get: the chat when Found is set.
type AIChatFound struct {
	Chat  api.ChatDetail `json:"chat"`
	Found bool           `json:"found"`
}

// AIAskArgs is one model call: the prompt and its options. Session names the extension's session the call belongs
// to; empty means a one-shot call. The same args open a stream.
type AIAskArgs struct {
	Prompt  string        `json:"prompt"`
	Options api.AIOptions `json:"options"`
	Session string        `json:"session,omitempty"`
}

// AIDecideArgs is one decision request.
type AIDecideArgs struct {
	Request api.DecisionRequest `json:"request"`
}

// AIStreamRef names an open stream.
type AIStreamRef struct {
	ID string `json:"id"`
}

// AIStreamOpened is the answer to stream-open.
type AIStreamOpened struct {
	ID string `json:"id"`
}

// AIStreamBatch is the answer to stream-next: the events since the last pull. Ended is set on the last batch; Text
// is then the full reply, and Failure says how the reply ended when it did not finish.
type AIStreamBatch struct {
	Events  []api.AIStreamEvent `json:"events,omitempty"`
	Ended   bool                `json:"ended,omitempty"`
	Text    string              `json:"text,omitempty"`
	Failure *api.Failure        `json:"failure,omitempty"`
}

// AIAvailability is the answer to is-available.
type AIAvailability struct {
	Available bool `json:"available"`
}

// AIPrepareArgs names the tier whose route is warmed.
type AIPrepareArgs struct {
	Options api.AIOptions `json:"options"`
}

// AIClipboardArgs asks for the clipboard as attachments; As selects how it is read.
type AIClipboardArgs struct {
	As string `json:"as,omitempty"`
}

// AIOCRImageArgs asks for the text of one image file as an attachment.
type AIOCRImageArgs struct {
	Path  string `json:"path"`
	Title string `json:"title,omitempty"`
}

// AIAttachments is the answer to the attachment operations.
type AIAttachments struct {
	Attachments []api.Attachment `json:"attachments"`
}
