package api

import "encoding/json"

// ModelTier selects a route by intent. The backend maps tiers to models and applies the user's
// entitlement. Extension code never names a model. The empty tier means smart.
type ModelTier string

const (
	ModelSmart ModelTier = "smart"
	ModelFast  ModelTier = "fast"
)

// Attachment is context added to a prompt. Kind is text, image or file. Path names a file on disk and must be
// absolute; DataURL carries an image inline. As selects how a file is read: text, image, metadata or ocr.
type Attachment struct {
	Kind     string   `json:"kind"`
	Title    string   `json:"title,omitempty"`
	Text     string   `json:"text,omitempty"`
	Path     string   `json:"path,omitempty"`
	DataURL  string   `json:"dataUrl,omitempty"`
	MimeType string   `json:"mimeType,omitempty"`
	File     *FileRef `json:"file,omitempty"`
	As       string   `json:"as,omitempty"`
	OCR      bool     `json:"ocr,omitempty"`
}

// AIOptions configures one call. Attachments are declared with the attachment builders.
type AIOptions struct {
	Model       ModelTier    `json:"model,omitempty"`
	System      string       `json:"system,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Stream event types. They carry the names the 1.x extension API uses, so an extension ported from 1.x
// keeps its switch statements. The host never sends a tool event: extensions cannot run tools.
const (
	AIEventStart     = "start"
	AIEventDelta     = "delta"
	AIEventToolStart = "tool_start"
	AIEventToolEnd   = "tool_end"
	AIEventDone      = "done"
	AIEventAborted   = "aborted"
	AIEventError     = "error"
)

// AIStreamEvent is one streaming event. Type is one of the AIEvent constants. Text is set on delta events,
// Message on error events.
type AIStreamEvent struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Name    string `json:"name,omitempty"`
	IsError bool   `json:"isError,omitempty"`
	Message string `json:"message,omitempty"`
}

// AIStream delivers events on Events and the full reply from Result. Events starts with an AIEventStart and
// ends with exactly one terminal event (done, aborted or error), then the channel is closed. The channel is
// buffered; a consumer that falls far behind loses deltas, but Result still holds the whole reply. Result
// blocks until the stream ends and returns an error for a failed or aborted reply. Abort ends the stream.
type AIStream interface {
	Events() <-chan AIStreamEvent
	Result() (string, error)
	Abort()
}

// AttachmentBuilder normalises host context into attachments. Each method reads host state and
// returns an error instead of a silent empty value.
type AttachmentBuilder interface {
	Text(text, title string) Attachment
	Image(pathOrDataURL string, title string, ocr bool) Attachment
	File(path string, as string, title string) Attachment
	SelectedText(title string) (Attachment, error)
	SelectedFiles(as string) ([]Attachment, error)
	Clipboard(as string) ([]Attachment, error)
	OCRImage(path string, title string) (Attachment, error)
}

// AISession is a conversation scoped to one extension. Session ids are local to the extension. The session
// keeps its last turns; Reset forgets them.
type AISession interface {
	Ask(prompt string, options AIOptions) (string, error)
	Stream(prompt string, options AIOptions) (AIStream, error)
	Reset() error
}

// DecisionQuestionType is predicate (yes/no), choice (one of Options) or score (integer scale).
type DecisionQuestionType string

const (
	DecisionPredicate DecisionQuestionType = "predicate"
	DecisionChoice    DecisionQuestionType = "choice"
	DecisionScore     DecisionQuestionType = "score"
)

// DecisionQuestion is one question answered against a shared input in a single batched call.
type DecisionQuestion struct {
	ID       string               `json:"id"`
	Type     DecisionQuestionType `json:"type"`
	Question string               `json:"question"`
	// Options has two to 32 distinct values for choice questions.
	Options []string `json:"options,omitempty"`
	// Min and Max bound score questions; at most ten levels. Pointers, so a minimum of zero is sent.
	Min    *int              `json:"min,omitempty"`
	Max    *int              `json:"max,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// DecisionRequest carries the shared input (a JSON string, object or array) and the questions. Images in
// Attachments are never dropped: an unreadable or unsupported one fails the call.
type DecisionRequest struct {
	Input       json.RawMessage    `json:"input"`
	Attachments []Attachment       `json:"attachments,omitempty"`
	Questions   []DecisionQuestion `json:"questions"`
}

// DecisionAnswer is the typed answer to one question. Predicate uses Flag and Probability; choice
// uses Choice; score uses Score. Probabilities and Confidence are present only when reported.
type DecisionAnswer struct {
	Type          DecisionQuestionType `json:"type"`
	Flag          bool                 `json:"flag,omitempty"`
	Probability   float64              `json:"probability,omitempty"`
	Choice        string               `json:"choice,omitempty"`
	Score         float64              `json:"score,omitempty"`
	Probabilities map[string]float64   `json:"probabilities,omitempty"`
	Confidence    *float64             `json:"confidence,omitempty"`
}

// DecisionResult answers every question. Usage is reported in tokens.
type DecisionResult struct {
	ID      string                    `json:"id"`
	Model   string                    `json:"model"`
	Answers map[string]DecisionAnswer `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
	} `json:"usage"`
}

// AI is the model access surface. Calls are quota-limited per extension (30 calls a minute: ask, stream,
// decide and session calls) and need the ai capability for review. Images are never silently dropped: an
// unsupported image route fails the call.
type AI interface {
	IsAvailable() (bool, error)
	Decide(request DecisionRequest) (DecisionResult, error)
	Ask(prompt string, options AIOptions) (string, error)
	Stream(prompt string, options AIOptions) (AIStream, error)
	// Prepare warms the model route before latency-sensitive work.
	Prepare(options AIOptions) error
	Attachments() AttachmentBuilder
	Session(id string, options AIOptions) (AISession, error)
	// Chats reads and opens the host's chats: the conversations the AI Chats page lists. Each is kept by the host.
	Chats() AIChats
}

// ChatAction is the payload of an ActionChat. ChatID names the chat; Message and Images are what a Continue sends; Prompt
// is the first message of a New chat; Tier is the tier a Tier change selects.
type ChatAction struct {
	ChatID  string      `json:"chatId,omitempty"`
	Message string      `json:"message,omitempty"`
	Images  []ChatImage `json:"images,omitempty"`
	Prompt  string      `json:"prompt,omitempty"`
	Tier    ModelTier   `json:"tier,omitempty"`
}

// BuilderAction is the payload of an ActionBuilder. ChatID names a builder chat, Prompt starts or seeds one, and
// ExtensionID names the extension a tweak changes.
type BuilderAction struct {
	ChatID      string `json:"chatId,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	ExtensionID string `json:"extensionId,omitempty"`
}

// ChatDetail is one chat with its transcript, as the host keeps it.
type ChatDetail struct {
	ChatSummary
	Messages []ChatMessage `json:"messages"`
}

// AIChats lists chats and returns the actions that open, start, continue, stop, re-tier or remove them. Reads are answered
// by the host; the actions run when the user picks them, on the palette.
type AIChats interface {
	List() []ChatSummary
	Get(chatID string) (ChatDetail, bool)
	New(prompt string) Action
	Open(chatID string) Action
	Continue(chatID, message string) Action
	Stop(chatID string) Action
	Tier(chatID string, tier ModelTier) Action
	Remove(chatID string) Action
}

// ChatSummary is one builder conversation. Only built-in extensions may call AIBuilder.
type ChatSummary struct {
	ID string `json:"id"`
	// Kind is conversation, builder or proofread.
	Kind  string    `json:"kind,omitempty"`
	Tier  ModelTier `json:"tier,omitempty"`
	Title string    `json:"title,omitempty"`
	// ExtensionID is the extension a builder chat wrote or was bound to, empty for other chats.
	ExtensionID    string   `json:"extensionId,omitempty"`
	Query          string   `json:"query,omitempty"`
	Status         string   `json:"status,omitempty"`
	CreatedAtMs    int64    `json:"createdAtMs,omitempty"`
	UpdatedAtMs    int64    `json:"updatedAtMs,omitempty"`
	ExtensionFiles []string `json:"extensionFiles"`
}

// TweakRequest asks the builder to change an existing extension.
type TweakRequest struct {
	ExtensionFile string `json:"extensionFile,omitempty"`
	ExtensionID   string `json:"extensionId,omitempty"`
	Title         string `json:"title,omitempty"`
	Prompt        string `json:"prompt,omitempty"`
}

// AIBuilder opens and manages extension-builder conversations. It is host-only: the built-in
// nevermind.ai-builder extension is its sole caller in v1.
type AIBuilder interface {
	StartChat(prompt, title string) Action
	OpenChat(chatID string) Action
	RemoveChat(chatID string) Action
	TweakExtension(request TweakRequest) Action
	OpenChatsList(title string) Action
	ListChats() []ChatSummary
	CleanupExpiredConversations() (removed int, err error)
	GetChat(chatID string) (ChatDetail, bool)
	// ExtensionPreview is what the extension shows now: its root rows and its commands, as a list view with the
	// side-preview presentation. It is the Side of a builder chat's transcript, and false when the extension is not
	// installed.
	ExtensionPreview(extensionID string) (View, bool)
}

// Ownership says which builder conversation wrote which extension file. It is host-only, and it is
// the mechanism that lets a conversation edit only its own extension.
type Ownership interface {
	OwnerOf(extensionFile string) (chatID string, found bool)
	FilesForChat(chatID string) []string
	CanWrite(extensionFile, chatID string) bool
}
