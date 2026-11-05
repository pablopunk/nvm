package protocol

import "github.com/pablopunk/nvm/native/internal/ext/api"

// WriteTextArgs puts text on the clipboard. Concealed asks clipboard history to skip it.
type WriteTextArgs struct {
	Text      string `json:"text"`
	Concealed bool   `json:"concealed,omitempty"`
}

// SelectionTextResult is the selection of the source window: its text, and Found false when nothing was selected.
type SelectionTextResult struct {
	Text  string `json:"text"`
	Found bool   `json:"found"`
}

// ReplaceTextArgs is the text that replaces the selection of the source window.
type ReplaceTextArgs struct {
	Text string `json:"text"`
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

// NotifyArgs is one operating-system notification: a title and a body, both plain text.
type NotifyArgs struct {
	Title string `json:"title"`
	Body  string `json:"body"`
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

// ClearArgs filters a history clear. Empty Types and zero OlderThanMs clear every entry.
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

// WriteImageArgs puts a PNG on the clipboard as a data URL. Concealed asks clipboard history to skip it.
type WriteImageArgs struct {
	DataURL   string `json:"dataUrl"`
	Concealed bool   `json:"concealed,omitempty"`
}

// ImageResult is a clipboard image as a PNG data URL, or Found false when the clipboard holds no image.
type ImageResult struct {
	DataURL string `json:"dataUrl,omitempty"`
	Found   bool   `json:"found"`
}

// FindArgs names the folders of one walk and its options.
type FindArgs struct {
	Roots   []string        `json:"roots"`
	Options api.FindOptions `json:"options"`
}

// ThumbnailResult is a picture of a file as a PNG data URL, or Found false when the file has no picture.
type ThumbnailResult struct {
	URL   string `json:"url,omitempty"`
	Found bool   `json:"found"`
}

// WriteBytesArgs names a file to write and its content. Overwrite replaces an existing file.
type WriteBytesArgs struct {
	Path      string `json:"path"`
	Data      []byte `json:"data"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

// TypeTextArgs types text into the frontmost app. Options carries the delay between characters.
type TypeTextArgs struct {
	Text    string          `json:"text"`
	Options api.TypeOptions `json:"options"`
}

// PathsArgs names several files or folders on this computer, such as the apps a running-state call asks about.
type PathsArgs struct {
	Paths []string `json:"paths"`
}

// PrefixArgs names the action IDs a shortcut read covers: those that start with Prefix.
type PrefixArgs struct {
	Prefix string `json:"prefix"`
}

// UninstallTrashArgs names a reviewed uninstall by its token and the items of it to move to the Trash.
type UninstallTrashArgs struct {
	Token string   `json:"token"`
	IDs   []string `json:"ids"`
}

// WriteFileTextArgs names a text file to write and its content. Overwrite replaces an existing file.
type WriteFileTextArgs struct {
	Path      string `json:"path"`
	Text      string `json:"text"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

// IndexArgs carries the options of an index read or rebuild.
type IndexArgs struct {
	Options api.IndexOptions `json:"options"`
}

// SearchIndexArgs names the text to match in the file index and the options that filter the match.
type SearchIndexArgs struct {
	Query   string           `json:"query"`
	Options api.IndexOptions `json:"options"`
}

// OpenWithArgs names a file and the application to open it with, by its bundle path.
type OpenWithArgs struct {
	Path    string `json:"path"`
	AppPath string `json:"appPath"`
}

// WriteHTMLArgs puts HTML and its plain-text flavor on the clipboard.
type WriteHTMLArgs struct {
	HTML      string `json:"html"`
	Text      string `json:"text"`
	Concealed bool   `json:"concealed,omitempty"`
}

// WriteFilesArgs puts a list of file paths on the clipboard, as a file manager copy does.
type WriteFilesArgs struct {
	Paths     []string `json:"paths"`
	Concealed bool     `json:"concealed,omitempty"`
}

// WriteImageFileArgs puts the PNG file at Path on the clipboard.
type WriteImageFileArgs struct {
	Path      string `json:"path"`
	Concealed bool   `json:"concealed,omitempty"`
}

// ImageFileResult is a clipboard image written to a file the host owns, or Found false when there is no image.
type ImageFileResult struct {
	File  api.FileRef `json:"file"`
	Found bool        `json:"found"`
}

// FormatsArgs names the formats a Read takes, in order. Empty means every format.
type FormatsArgs struct {
	Formats []string `json:"formats,omitempty"`
}

// ContentArgs carries one ClipboardContent to write.
type ContentArgs struct {
	Content api.ClipboardContent `json:"content"`
}
