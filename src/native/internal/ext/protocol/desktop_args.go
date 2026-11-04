package protocol

import "github.com/pablopunk/nvm/native/internal/ext/api"

// WriteTextArgs puts text on the clipboard. Concealed asks clipboard history to skip it.
type WriteTextArgs struct {
	Text      string `json:"text"`
	Concealed bool   `json:"concealed,omitempty"`
}

// PathArgs names a file or folder on this computer.
type PathArgs struct {
	Path string `json:"path"`
}

// URLArgs names a web link.
type URLArgs struct {
	URL string `json:"url"`
}

// QueryArgs carries a search string.
type QueryArgs struct {
	Query string `json:"query"`
}

// CapabilityArgs names a public capability, as api.System.HasCapability does.
type CapabilityArgs struct {
	Name string `json:"name"`
}

// IconResult is an app icon as a data URL, or Found false when the system has none.
type IconResult struct {
	DataURL string `json:"dataUrl,omitempty"`
	Found   bool   `json:"found"`
}

// FocusArgs names one window handle from a listing.
type FocusArgs struct {
	Handle api.WindowHandle `json:"handle"`
}

// ApplyToArgs places one listed window.
type ApplyToArgs struct {
	Handle  api.WindowHandle            `json:"handle"`
	Layout  api.WindowLayout            `json:"layout"`
	Options api.WindowManagementOptions `json:"options"`
}

// ClipboardGetResult is one history entry, or Found false when it is gone.
type ClipboardGetResult struct {
	Found bool               `json:"found"`
	Entry api.ClipboardEntry `json:"entry"`
}

// IDsArgs names history entries.
type IDsArgs struct {
	IDs []string `json:"ids"`
}

// ClearArgs filters a history clear. Empty Types and zero OlderThanMs clear every unpinned entry.
type ClearArgs struct {
	Types       []string `json:"types,omitempty"`
	OlderThanMs int64    `json:"olderThanMs,omitempty"`
}

// OCRImageArgs names an image file to read.
type OCRImageArgs struct {
	Input   api.OCRInput   `json:"input"`
	Options api.OCROptions `json:"options"`
}

// OCRScreenArgs reads the whole screen.
type OCRScreenArgs struct {
	Options api.OCROptions `json:"options"`
}

// OCRRegionArgs reads one rectangle of the screen, in physical screen coordinates.
type OCRRegionArgs struct {
	Region  api.Rect       `json:"region"`
	Options api.OCROptions `json:"options"`
}

// ShellExecArgs runs one program with its arguments.
type ShellExecArgs struct {
	Command string           `json:"command"`
	Args    []string         `json:"args,omitempty"`
	Options api.ShellOptions `json:"options"`
}

// ShellScriptArgs runs one script: shell source for Script, AppleScript source for AppleScript.
type ShellScriptArgs struct {
	Script  string           `json:"script"`
	Options api.ShellOptions `json:"options"`
}

// ShellWhichArgs names a program to look up on the user's PATH.
type ShellWhichArgs struct {
	Command string `json:"command"`
}
