package api

import "encoding/json"

// ModelTier selects a route by intent. The backend maps tiers to models and applies the user's
// entitlement. Extension code never names a model.
type ModelTier string

const (
	ModelSmart ModelTier = "smart"
	ModelFast  ModelTier = "fast"
)

// Attachment is context added to a prompt. Kind is text, image or file; As selects how a file
// is read: text, image, metadata or ocr.
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

// AIStreamEvent is one streaming event. Type is start, delta, tool-start, tool-end, done,
// aborted or error.
type AIStreamEvent struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Name    string `json:"name,omitempty"`
	IsError bool   `json:"isError,omitempty"`
	Message string `json:"message,omitempty"`
}

// AIStream delivers events on Events and the full text from Result. Events is closed when the
// stream ends. Cancelling the Context's Std() aborts the stream.
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

// AISession is a conversation scoped to one extension. Session ids are local to the extension.
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
	// Min and Max bound score questions; at most ten levels. Labels name levels inside the range.
	Min    int               `json:"min,omitempty"`
	Max    int               `json:"max,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// DecisionRequest carries the shared input (JSON text, object or array) and the questions.
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

// AI is the model access surface. Calls are quota-limited per extension and need the ai capability
// for review. Images are never silently dropped: an unsupported image route fails the call.
type AI interface {
	IsAvailable() (bool, error)
	Decide(request DecisionRequest) (DecisionResult, error)
	Ask(prompt string, options AIOptions) (string, error)
	Stream(prompt string, options AIOptions) (AIStream, error)
	// Prepare warms the model route before latency-sensitive work.
	Prepare(options AIOptions) error
	Attachments() AttachmentBuilder
	Session(id string, options AIOptions) (AISession, error)
}

// ChatSummary is one builder conversation. Only built-in extensions may call AIBuilder.
type ChatSummary struct {
	ID             string   `json:"id"`
	Title          string   `json:"title,omitempty"`
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
}

// Ownership says which builder conversation wrote which extension file. It is host-only, and it is
// the mechanism that lets a conversation edit only its own extension.
type Ownership interface {
	OwnerOf(extensionFile string) (chatID string, found bool)
	FilesForChat(chatID string) []string
	CanWrite(extensionFile, chatID string) bool
}
