package api

import (
	"fmt"
	"strings"
)

// GridSpec tunes a grid view. Grids show Items as tiles.
type GridSpec struct {
	Columns     int    `json:"columns,omitempty"`
	Layout      string `json:"layout,omitempty"` // square, wide or compact
	AspectRatio string `json:"aspectRatio,omitempty"`
	// CategoryKeys makes Tab and Shift+Tab step the view's Filter: the next or previous option, wrapping, with
	// its OnChange run as a pick of its menu would run it. The footer button shows the chosen option.
	// Without a Filter with an OnChange, the keys do nothing.
	CategoryKeys bool `json:"categoryKeys,omitempty"`
	// SelectionFooter shows the selected item's title and subtitle in the footer. The view's subtitle is not repeated
	// there: a Filter already names the category.
	SelectionFooter bool `json:"selectionFooter,omitempty"`
}

// CrudItem is one record in a host-owned collection. Unlike Item it has no free-form action
// lists: the host wires Preview (Enter), Edit (Cmd+E) and Remove (Cmd+Backspace, confirmed).
type CrudItem struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Subtitle    string      `json:"subtitle,omitempty"`
	Icon        string      `json:"icon,omitempty"`
	Image       *Image      `json:"image,omitempty"`
	Accessories []Accessory `json:"accessories,omitempty"`
	Keywords    []string    `json:"keywords,omitempty"`
	Aliases     []string    `json:"aliases,omitempty"`
	Detail      *Detail     `json:"detail,omitempty"`
	Preview     *Action     `json:"preview,omitempty"`
	Edit        *Action     `json:"edit,omitempty"`
	Remove      *Action     `json:"remove,omitempty"`
}

// CollectionSpec is a generic CRUD list: search chrome plus consistent add, preview, edit and
// remove behaviour. Supply only the operations the data model supports.
type CollectionSpec struct {
	SearchPlaceholder string     `json:"searchPlaceholder,omitempty"`
	EmptyTitle        string     `json:"emptyTitle,omitempty"`
	EmptySubtitle     string     `json:"emptySubtitle,omitempty"`
	Add               *Action    `json:"add,omitempty"`
	Items             []CrudItem `json:"items"`
}

// Validate rejects reserved and duplicate record ids.
func (c CollectionSpec) Validate() error {
	seen := map[string]bool{}
	for _, record := range c.Items {
		if record.ID == "" || strings.HasPrefix(record.ID, reservedRowPrefix) {
			return fmt.Errorf("collection record id %q is empty or reserved", record.ID)
		}
		if seen[record.ID] {
			return fmt.Errorf("duplicate collection record id %q", record.ID)
		}
		seen[record.ID] = true
	}
	return nil
}

// reservedRowPrefix is used by the host for its own rows (add, empty state); extensions must not
// produce ids with it.
const reservedRowPrefix = "__nvm:"

// PreviewSpec is a full-screen read-only preview of content the extension already has.
// Markdown needs blank lines between paragraphs; a single newline does not start a new line.
type PreviewSpec struct {
	Markdown string `json:"markdown,omitempty"`
	Text     string `json:"text,omitempty"`
	Image    *Image `json:"image,omitempty"`
	Video    string `json:"video,omitempty"`
}

// FilePreview asks the host to preview a file: media renders natively, text shows its leading
// 64 KB, binaries are summarised. The host reads the file; the extension never reads it for this.
type FilePreview struct {
	File FileRef `json:"file"`
}

// EditorSpec is an editable text or Markdown surface. The host injects the final text into the
// submit action as Invocation.EditorContent.
type EditorSpec struct {
	Content       string        `json:"content,omitempty"`
	Format        string        `json:"format,omitempty"` // text or markdown
	Language      string        `json:"language,omitempty"`
	Placeholder   string        `json:"placeholder,omitempty"`
	ReadOnly      bool          `json:"readOnly,omitempty"`
	LivePreview   bool          `json:"livePreview,omitempty"`
	TitleFromLine bool          `json:"titleFromLine,omitempty"`
	Draft         *DraftOptions `json:"draft,omitempty"`
	Submit        *Action       `json:"submit,omitempty"`
}

// DraftOptions makes the host autosave unsaved editor text and recover it on reopen. Drafts are
// unsaved-work buffers; the extension's storage stays the source of truth.
type DraftOptions struct {
	Key        string  `json:"key"`
	Version    string  `json:"version"`
	DebounceMs int64   `json:"debounceMs,omitempty"`
	Autosave   *Action `json:"autosave,omitempty"`
	// OnConflict receives Invocation.DraftConflict when the stored draft has another version.
	OnConflict *ActionRef `json:"onConflict,omitempty"`
}

// ProgressStatus is the marker shown for one progress step.
type ProgressStatus string

const (
	StepPending  ProgressStatus = "pending"
	StepActive   ProgressStatus = "in-progress"
	StepComplete ProgressStatus = "complete"
	StepFailed   ProgressStatus = "error"
)

// ProgressStep is one step in a progress view.
type ProgressStep struct {
	Title  string         `json:"title"`
	Status ProgressStatus `json:"status,omitempty"`
}

// ProgressSpec is a step or determinate progress view. Value counts completed work, not the
// current step, so pair it with Total.
type ProgressSpec struct {
	Steps  []ProgressStep `json:"steps,omitempty"`
	Value  int            `json:"value,omitempty"`
	Total  int            `json:"total,omitempty"`
	Label  string         `json:"label,omitempty"`
	Status string         `json:"status,omitempty"`
}

// CameraSpec is a live camera view. Capture runs only while the view is visible.
type CameraSpec struct {
	DeviceID           string `json:"deviceId,omitempty"`
	ShowDeviceSwitcher *bool  `json:"showDeviceSwitcher,omitempty"` // default true
	Muted              *bool  `json:"muted,omitempty"`              // default true, as the Electron view
	Controls           bool   `json:"controls,omitempty"`
	Mirrored           bool   `json:"mirrored,omitempty"`
	Framing            string `json:"framing,omitempty"` // cover (default) or contain
}

// WebviewPermission is an explicit opt-in for an iframe capability. Autoplay is the default.
type WebviewPermission string

const (
	WebviewAutoplay       WebviewPermission = "autoplay"
	WebviewCamera         WebviewPermission = "camera"
	WebviewMicrophone     WebviewPermission = "microphone"
	WebviewDisplayCapture WebviewPermission = "display-capture"
	WebviewClipboardRead  WebviewPermission = "clipboard-read"
	WebviewClipboardWrite WebviewPermission = "clipboard-write"
)

// WebviewSpec is a sandboxed HTML iframe with no Node access. Use it only when host primitives
// do not fit: it is the last resort, not a way to rebuild forms.
type WebviewSpec struct {
	HTML        string              `json:"html"`
	Permissions []WebviewPermission `json:"permissions,omitempty"`
}

// ChatMessage is one turn in a chat view. Role is user, assistant or system. Thinking is a reasoning block that
// opens on request, Images are pictures sent with a user turn, Tool is a tool row, and Streaming marks an assistant
// turn still arriving: the host shows progress for it and the chat's Stop action.
type ChatMessage struct {
	Role string `json:"role"`
	// Title names a system row, such as a refusal's notice; empty shows it as Note.
	Title     string      `json:"title,omitempty"`
	Content   string      `json:"content"`
	Thinking  string      `json:"thinking,omitempty"`
	Images    []ChatImage `json:"images,omitempty"`
	Tool      *ChatTool   `json:"tool,omitempty"`
	Streaming bool        `json:"streaming,omitempty"`
}

// ChatImage is a picture in a chat turn, as a data URL.
type ChatImage struct {
	Title   string `json:"title,omitempty"`
	DataURL string `json:"dataUrl"`
}

// ChatTool is transient activity: running or waiting tools share one live status slot; done and failed tools stay
// out of the visible conversation. Result describes an outcome or approval request. Submit an empty message to allow
// a waiting tool. Protocol records may remain in Messages for the model.
type ChatTool struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
}

// NewChatView is the transcript of a chat that has no record yet: a draft with no messages and no tier picker. Submit
// runs with the typed text in Invocation.Form["message"] and the pictures in Invocation.Images, and its handler starts
// the chat, which the host then opens. The chat starts on tier.
func NewChatView(title string, tier ModelTier, submit ActionRef) View {
	return View{Kind: ViewChat, Title: title, Chat: &ChatSpec{Tier: tier, Submit: &submit}}
}

// ChatSpec is a transcript view. The host draws the messages, the composer and the streaming progress. Submit runs
// with the typed text in Invocation.Form["message"] (empty when the user only allows a waiting tool). Stop runs
// while a message streams. Tier is the selected model tier and OnTier runs with the tier the user chose in
// Ref.Args["tier"]; without OnTier the picker is hidden.
type ChatSpec struct {
	Messages    []ChatMessage `json:"messages"`
	Placeholder string        `json:"placeholder,omitempty"`
	Tier        ModelTier     `json:"tier,omitempty"`
	OnTier      *ActionRef    `json:"onTier,omitempty"`
	Submit      *ActionRef    `json:"submit,omitempty"`
	Stop        *ActionRef    `json:"stop,omitempty"`
	// Attach shows the composer's picture control. A submit then carries the pictures in Invocation.Images.
	Attach bool `json:"attach,omitempty"`

	// Side is a view drawn beside the transcript, as the AI Chats page draws the preview of a builder's extension. The
	// host hides and shows it with a palette action, and keeps the side's place while the owner returns the same view.
	Side *View `json:"side,omitempty"`

	// SideExtension names the extension whose actions the rows of Side run. Only a built-in may set it; otherwise they run
	// in the extension that returned the view.
	SideExtension string `json:"sideExtension,omitempty"`
}
