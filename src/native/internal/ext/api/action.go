package api

import (
	"errors"
	"fmt"
	"net/url"
)

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
	ActionShellExec     ActionKind = "shell-exec"
	ActionToggleSetting ActionKind = "toggle-setting"
	ActionWindow        ActionKind = "window"
	ActionSystem        ActionKind = "system"
	ActionUpdate        ActionKind = "update"
	ActionCamera        ActionKind = "camera"
	ActionOpenPalette   ActionKind = "open-palette"
	ActionSetQuery      ActionKind = "set-query"
)

// Op values for the kinds that have sub-operations.
var (
	windowOps = []string{"create", "show", "hide", "toggle", "close"}
	systemOps = []string{"lock-screen", "sleep", "restart", "shut-down", "log-out", "empty-trash", "quit", "open-system-settings", "open-keyboard-settings"}
	updateOps = []string{"check", "download", "install"}
	cameraOps = []string{"switch-device", "next-device", "previous-device", "toggle-muted", "toggle-controls"}
)

// Action is one declarative operation attached to an item, view, command or menu row. Helper
// constructors in the author package fill the fields that each kind requires; Validate checks them.
type Action struct {
	Kind       ActionKind `json:"kind"`
	Op         string     `json:"op,omitempty"`
	Title      string     `json:"title,omitempty"`
	Subtitle   string     `json:"subtitle,omitempty"`
	Icon       string     `json:"icon,omitempty"`
	Style      string     `json:"style,omitempty"`
	Shortcut   string     `json:"shortcut,omitempty"`
	Background bool       `json:"background,omitempty"`
	// DismissAfterRun closes the palette once the action is dispatched.
	DismissAfterRun bool          `json:"dismissAfterRun,omitempty"`
	Confirm         *Confirmation `json:"confirm,omitempty"`
	// Ref names the handler for ActionRun.
	Ref *ActionRef `json:"ref,omitempty"`
	// Payload fields used by the declarative kinds. Only the ones the kind documents are read.
	Path    string            `json:"path,omitempty"`
	Paths   []string          `json:"paths,omitempty"`
	URL     string            `json:"url,omitempty"`
	Text    string            `json:"text,omitempty"`
	Query   string            `json:"query,omitempty"`
	App     string            `json:"app,omitempty"`
	Setting string            `json:"setting,omitempty"`
	Content *ClipboardContent `json:"content,omitempty"`
	Paste   *PasteOptions     `json:"paste,omitempty"`
	Shell   *ShellCommand     `json:"shell,omitempty"`
	View    *View             `json:"view,omitempty"`
	Window  *WindowRef        `json:"window,omitempty"`
	Submenu *ActionPanel      `json:"submenu,omitempty"`
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
	Type         string   `json:"type"`
	Text         string   `json:"text,omitempty"`
	HTML         string   `json:"html,omitempty"`
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
		if a.Content == nil || !oneOf(a.Content.Type, "text", "html", "image", "files") {
			return errors.New("content with type text, html, image or files is required")
		}
	case ActionTrash:
		if len(a.Paths) == 0 {
			return errors.New("paths are required")
		}
	case ActionPushView, ActionReplaceView, ActionRootView:
		if a.View == nil {
			return errors.New("view is required")
		}
	case ActionPop, ActionOpenPalette, ActionSetQuery:
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
	case ActionCamera:
		return validateOp(a.Op, cameraOps)
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
