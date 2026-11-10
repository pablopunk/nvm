package api

import (
	"errors"
	"fmt"
	"net/url"
)

// HostToggleMenuBar is the ActionHost op that shows or hides the menu bar items of the extension whose row runs it.
const HostToggleMenuBar = "toggle-menu-bar"

// ActionKind selects what the host does when an action runs. Only ActionRun executes extension
// code; every other kind is a declarative host operation that the host implements.
type ActionKind string

const (
	ActionRun           ActionKind = "run"
	ActionOpenPath      ActionKind = "open-path"
	ActionRevealPath    ActionKind = "reveal-path"
	ActionQuickLook     ActionKind = "quick-look"
	ActionOpenWith      ActionKind = "open-with"
	ActionOpenURL       ActionKind = "open-url"
	ActionCopyText      ActionKind = "copy-text"
	ActionCopyImage     ActionKind = "copy-image"
	ActionPasteText     ActionKind = "paste-text"
	ActionPaste         ActionKind = "paste"
	ActionTypeText      ActionKind = "type-text"
	ActionTrash         ActionKind = "trash"
	ActionPushView      ActionKind = "push-view"
	ActionReplaceView   ActionKind = "replace-view"
	ActionRootView      ActionKind = "root-view"
	ActionPop           ActionKind = "pop"
	ActionRefreshView   ActionKind = "refresh-view"
	ActionShellExec     ActionKind = "shell-exec"
	ActionToggleSetting ActionKind = "toggle-setting"
	ActionWindow        ActionKind = "window"
	ActionSystem        ActionKind = "system"
	ActionUpdate        ActionKind = "update"
	ActionCamera        ActionKind = "camera"
	ActionOpenPalette   ActionKind = "open-palette"
	ActionSetQuery      ActionKind = "set-query"
	// ActionAccount runs an account operation the host owns, such as sign-in.
	ActionAccount ActionKind = "account"
	// ActionHost runs a host operation, such as opening a host page. Op is one of hostOps: HostToggleMenuBar acts on the
	// extension whose row it is, and the others are for built-ins.
	ActionHost ActionKind = "host"
	// ActionChat opens, starts, continues or removes a host-owned chat. Chat holds its payload.
	ActionChat ActionKind = "chat"
	// ActionBuilder opens, starts, tweaks or removes an AI builder chat, or opens the list of them. Builder holds its
	// payload. Only built-in extensions may return it.
	ActionBuilder ActionKind = "builder"
	// ActionRecordPaletteShortcut opens the host's recorder for the palette shortcut. Key capture stays with the host.
	ActionRecordPaletteShortcut ActionKind = "record-palette-shortcut"
	// ActionShortcut changes the user's shortcut for the action Target names: Op record opens the host's recorder for it,
	// and remove drops the shortcut the user chose. Key capture stays with the host.
	ActionShortcut ActionKind = "shortcut"
	// ActionUnavailable is a placeholder for an operation this host does not offer. It validates, and the palette
	// shows "This action is not available yet" when it runs.
	ActionUnavailable ActionKind = "unavailable"
)

// AccountSignIn starts the sign-in flow.
const AccountSignIn = "sign-in"

// Ops of the chat action. Open shows a chat, New starts one with a prompt, Continue sends a message and shows the chat
// (an empty message allows a tool call that waits), Stop ends a reply that streams, Tier changes the chat's model tier,
// Remove deletes a chat the user asked to remove, Again drops the last reply and asks the last question again, and
// Allow and Refuse answer a tool call that waits.
const (
	ChatOpen     = "open"
	ChatNew      = "new"
	ChatContinue = "continue"
	ChatStop     = "stop"
	ChatTier     = "tier"
	ChatRemove   = "remove"
	ChatAgain    = "again"
	ChatAllow    = "allow"
	ChatRefuse   = "refuse"
)

// Ops of the builder action: Start begins a builder chat with a prompt, Open shows one, Remove deletes one, Tweak opens
// a chat that changes an extension, List shows the builder chats, and Review opens the review of an extension.
const (
	BuilderStart  = "start"
	BuilderOpen   = "open"
	BuilderRemove = "remove"
	BuilderTweak  = "tweak"
	BuilderList   = "list"
	BuilderReview = "review"
)

// Op values for the kinds that have sub-operations.
var (
	windowOps   = []string{"create", "show", "hide", "toggle", "close"}
	systemOps   = []string{"lock-screen", "sleep", "restart", "restart-app", "shut-down", "log-out", "empty-trash", "quit", "open-system-settings", "open-keyboard-settings"}
	updateOps   = []string{UpdateOpCheck, UpdateOpDownload, UpdateOpInstall, UpdateOpChannel}
	accountOps  = []string{AccountSignIn}
	chatOps     = []string{ChatOpen, ChatNew, ChatContinue, ChatStop, ChatTier, ChatRemove, ChatAgain, ChatAllow, ChatRefuse}
	builderOps  = []string{BuilderStart, BuilderOpen, BuilderRemove, BuilderTweak, BuilderList, BuilderReview}
	hostOps     = []string{"config-folder", "dictation-settings", "updates", "account-page", HostToggleMenuBar}
	cameraOps   = []string{"switch-device", "next-device", "previous-device", "toggle-muted", "toggle-controls"}
	shortcutOps = []string{ShortcutOpRecord, ShortcutOpRemove}
)

// Action is one declarative operation attached to an item, view, command or menu row. Helper
// constructors in the author package fill the fields that each kind requires; Validate checks them.
type Action struct {
	// Requires lists all operations this command, row or action needs. API 1.1; grants no access.
	Requires   []Operation `json:"requires,omitempty"`
	Kind       ActionKind  `json:"kind"`
	Op         string      `json:"op,omitempty"`
	Title      string      `json:"title,omitempty"`
	Subtitle   string      `json:"subtitle,omitempty"`
	Icon       string      `json:"icon,omitempty"`
	Style      string      `json:"style,omitempty"`
	Shortcut   string      `json:"shortcut,omitempty"`
	Background bool        `json:"background,omitempty"`
	// Preview marks an action that previews the item's content. The menu lists it and Enter never runs it, so a row that
	// wants Enter to preview sets its Primary. The host adds no generic Preview option beside an action marked so.
	Preview bool `json:"preview,omitempty"`
	// DismissAfterRun closes the palette once the action is dispatched.
	DismissAfterRun bool          `json:"dismissAfterRun,omitempty"`
	Confirm         *Confirmation `json:"confirm,omitempty"`
	// Ref names the handler for ActionRun.
	Ref *ActionRef `json:"ref,omitempty"`
	// Payload fields used by the declarative kinds. Only the ones the kind documents are read.
	Path    string   `json:"path,omitempty"`
	Paths   []string `json:"paths,omitempty"`
	URL     string   `json:"url,omitempty"`
	Text    string   `json:"text,omitempty"`
	Query   string   `json:"query,omitempty"`
	App     string   `json:"app,omitempty"`
	Setting string   `json:"setting,omitempty"`
	// Target names the action a shortcut kind changes: its action ID.
	Target  string            `json:"target,omitempty"`
	Content *ClipboardContent `json:"content,omitempty"`
	Paste   *PasteOptions     `json:"paste,omitempty"`
	Typing  *TypeOptions      `json:"typing,omitempty"`
	Shell   *ShellCommand     `json:"shell,omitempty"`
	View    *View             `json:"view,omitempty"`
	Window  *WindowRef        `json:"window,omitempty"`
	Submenu *ActionPanel      `json:"submenu,omitempty"`
	Chat    *ChatAction       `json:"chat,omitempty"`
	Builder *BuilderAction    `json:"builder,omitempty"`
}

// ActionRef names a handler the extension registered. IDs are stable across runs because
// shortcuts, aliases and recents bind to them. Args are small strings the host echoes back
// untouched; anything richer should be stored by the extension and looked up by ID.
type ActionRef struct {
	ID   string            `json:"id"`
	Args map[string]string `json:"args,omitempty"`
}

// Confirmation makes the host ask before an action runs. It is a palette row, not a dialog.
type Confirmation struct {
	Title        string `json:"title,omitempty"`
	Message      string `json:"message,omitempty"`
	ConfirmLabel string `json:"confirmLabel,omitempty"`
	CancelLabel  string `json:"cancelLabel,omitempty"`
	Destructive  bool   `json:"destructive,omitempty"`
}

// ActionPanel is the grouped action menu (the Cmd+K list) for an item, view or command.
type ActionPanel struct {
	Title    string          `json:"title,omitempty"`
	Sections []ActionSection `json:"sections"`
}

// ActionSection is one titled group inside an ActionPanel.
type ActionSection struct {
	Title   string   `json:"title,omitempty"`
	Actions []Action `json:"actions"`
}

// ClipboardContent is a value written to the system clipboard or pasted.
type ClipboardContent struct {
	// Type is text, html, image, files, or empty when Read found nothing.
	Type         string   `json:"type"`
	Text         string   `json:"text,omitempty"`
	HTML         string   `json:"html,omitempty"`
	RTF          string   `json:"rtf,omitempty"`
	ImageDataURL string   `json:"imageDataUrl,omitempty"`
	ImagePath    string   `json:"imagePath,omitempty"`
	Paths        []string `json:"paths,omitempty"`
	Concealed    bool     `json:"concealed,omitempty"`
}

// PasteOptions tunes paste-text and paste. Zero values are the documented defaults.
type PasteOptions struct {
	KeepPaletteOpen     bool   `json:"keepPaletteOpen,omitempty"`
	RestoreClipboard    bool   `json:"restoreClipboard,omitempty"`
	PlainTextOnly       *bool  `json:"plainTextOnly,omitempty"`
	Concealed           bool   `json:"concealed,omitempty"`
	RestoreDelayMs      int64  `json:"restoreDelayMs,omitempty"`
	ExpectedFrontmostID string `json:"expectedFrontmostId,omitempty"`
}

// ShellCommand is a process to run. Set exactly one of Command (argv style) or Script.
// Declarative shell actions ask for confirmation, require system, and show captured output as plain text.
// Script uses the host login-script implementation (macOS/Linux); argv Command is portable.
type ShellCommand struct {
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Script      string            `json:"script,omitempty"`
	CWD         string            `json:"cwd,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	TimeoutMs   int64             `json:"timeoutMs,omitempty"`
	OutputLimit int64             `json:"outputLimit,omitempty"`
}

// Validate checks the fields each action kind needs. Handler existence is checked by the host
// when the extension is loaded, not here.
func (a Action) Validate() error {
	if err := validateRequirements(a.Requires); err != nil {
		return err
	}
	if a.Kind == ActionRun {
		if a.Ref == nil || a.Ref.ID == "" {
			return errors.New("run action needs a ref with an id")
		}
		return nil
	}
	if err := a.validateDeclarative(); err != nil {
		return fmt.Errorf("%s action: %w", a.Kind, err)
	}
	return nil
}

func (a Action) validateDeclarative() error {
	switch a.Kind {
	case ActionOpenPath, ActionRevealPath, ActionQuickLook, ActionOpenWith:
		if a.Path == "" {
			return errors.New("path is required")
		}
		if a.Kind == ActionOpenWith && a.App == "" {
			return errors.New("app is required")
		}
	case ActionOpenURL:
		parsed, err := url.Parse(a.URL)
		if err != nil || !oneOf(parsed.Scheme, "http", "https", "mailto") {
			return fmt.Errorf("url %q must be http, https or mailto", a.URL)
		}
	case ActionCopyText, ActionPasteText, ActionTypeText:
		if a.Text == "" {
			return errors.New("text is required")
		}
	case ActionCopyImage:
		if a.Path == "" && a.URL == "" {
			return errors.New("path or url is required")
		}
	case ActionPaste:
		return validatePasteContent(a.Content)
	case ActionTrash:
		if len(a.Paths) == 0 {
			return errors.New("paths are required")
		}
	case ActionPushView, ActionReplaceView, ActionRootView:
		if a.View == nil {
			return errors.New("view is required")
		}
	case ActionPop, ActionRefreshView, ActionOpenPalette, ActionSetQuery, ActionRecordPaletteShortcut, ActionUnavailable:
		return nil
	case ActionShellExec:
		return validateShell(a.Shell)
	case ActionToggleSetting:
		if a.Setting == "" {
			return errors.New("setting id is required")
		}
	case ActionWindow:
		return validateWindowAction(a)
	case ActionSystem:
		return validateOp(a.Op, systemOps)
	case ActionUpdate:
		return validateOp(a.Op, updateOps)
	case ActionShortcut:
		if a.Target == "" {
			return errors.New("a shortcut action needs the action it changes")
		}
		return validateOp(a.Op, shortcutOps)
	case ActionCamera:
		return validateOp(a.Op, cameraOps)
	case ActionAccount:
		return validateOp(a.Op, accountOps)
	case ActionHost:
		return validateOp(a.Op, hostOps)
	case ActionChat:
		if a.Chat == nil {
			return fmt.Errorf("chat action %q needs its chat payload", a.Title)
		}
		return validateOp(a.Op, chatOps)
	case ActionBuilder:
		if a.Builder == nil {
			return fmt.Errorf("builder action %q needs its builder payload", a.Title)
		}
		return validateOp(a.Op, builderOps)
	default:
		return fmt.Errorf("unknown action kind %q", a.Kind)
	}
	return nil
}

func validateOp(op string, allowed []string) error {
	for _, candidate := range allowed {
		if op == candidate {
			return nil
		}
	}
	return fmt.Errorf("op %q is not one of %v", op, allowed)
}

func validateShell(shell *ShellCommand) error {
	if shell == nil {
		return errors.New("shell is required")
	}
	if (shell.Command == "") == (shell.Script == "") {
		return errors.New("set exactly one of command or script")
	}
	return nil
}

func validateWindowAction(a Action) error {
	if err := validateOp(a.Op, windowOps); err != nil {
		return err
	}
	if a.Window == nil || (a.Window.ID == "" && a.Window.View == nil) {
		return errors.New("window id or view is required")
	}
	return nil
}

// validatePasteContent checks the clipboard content a paste action carries: its type, and the field that type needs.
func validatePasteContent(content *ClipboardContent) error {
	if content == nil || !oneOf(content.Type, "text", "html", "image", "files") {
		return errors.New("content with type text, html, image or files is required")
	}
	switch content.Type {
	case "image":
		if content.ImageDataURL == "" && content.ImagePath == "" {
			return errors.New("image content needs imageDataUrl or imagePath")
		}
	case "files":
		if len(content.Paths) == 0 {
			return errors.New("files content needs paths")
		}
	}
	return nil
}
