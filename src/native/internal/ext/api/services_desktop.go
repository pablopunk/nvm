package api

import "time"

// Desktop groups the OS-facing services. Declaring the matching capability is review metadata;
// the host does not gate these calls by declaration.
type Desktop interface {
	Keyboard() Keyboard
	Clipboard() SystemClipboard
	Selection() Selection
	Apps() Apps
	// WindowManagement returns nil on hosts without window management. Check
	// System().HasCapability("window-management") before calling it.
	WindowManagement() WindowManagement
	Files() Files
	Shell() Shell
}

// TypeOptions tunes typing into the frontmost app.
type TypeOptions struct {
	DelayMs int64 `json:"delayMs,omitempty"`
}

// Keyboard types text into the frontmost app without touching the clipboard. Undo sends the undo gesture (Command+Z
// or Ctrl+Z) to the app the palette was summoned from, and needs a replacement made with Selection.ReplaceText in that
// window first. Undo requires the desktop.keyboard capability.
type Keyboard interface {
	TypeText(text string, options TypeOptions) error
	Undo() error
}

// SystemClipboard reads and writes the OS clipboard directly. Concealed writes are kept out of
// Nevermind's history where the platform allows it.
type SystemClipboard interface {
	ReadText() (string, error)
	WriteText(text string, concealed bool) error
	ReadHTML() (string, error)
	WriteHTML(html, text string, concealed bool) error
	ReadImage() (dataURL string, found bool, err error)
	WriteImage(dataURL string, concealed bool) error
	ReadFiles() ([]FileRef, error)
	WriteFiles(paths []string, concealed bool) error
	Read(formats []string) (ClipboardContent, error)
	Write(content ClipboardContent) error
}

// SelectionState is the answer to Selection.Access, which never prompts.
type SelectionState string

const (
	SelectionAllowed     SelectionState = "allowed"
	SelectionDenied      SelectionState = "denied"
	SelectionUnavailable SelectionState = "unavailable"
	SelectionUnsupported SelectionState = "unsupported"
	SelectionUnknown     SelectionState = "unknown"
)

// SelectionAccess distinguishes denied permission, missing tools and unsupported sessions.
type SelectionAccess struct {
	State   SelectionState `json:"state"`
	Message string         `json:"message"`
}

// Selection reads and replaces the text selected in the app the palette was summoned from. It requires the
// desktop.selection capability. The clipboard is saved and put back by the host, and formatted clipboards (HTML, RTF,
// files, images) are refused rather than lost.
type Selection interface {
	Access() (SelectionAccess, error)
	// Text captures the selection after the palette releases focus. found is false when nothing
	// could be captured; that does not prove the source app has no selection.
	Text() (text string, found bool, err error)
	// ReplaceText pastes text over the selection of the source app. It returns false when the host could not send the
	// paste, and the text then stays on the clipboard. The paste lands in the app the selection came from.
	ReplaceText(text string) (replaced bool, err error)
	Files() ([]string, error)
}

// App is an installed application. ID is the platform identifier (bundle id on macOS); show Name.
type App struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	FirstSeenMs int64  `json:"firstSeenMs,omitempty"`
}

// Apps lists and launches applications from the host index.
type Apps interface {
	Frontmost() (App, error)
	Launch(path string) error
	List() ([]App, error)
	Search(query string) ([]App, error)
	Icon(path string) (dataURL string, found bool, err error)
}

// FocusResult reports a focus request. Failures arrive with OK false and a palette-ready Message.
type FocusResult struct {
	OK      bool   `json:"ok"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// WindowManagement moves, resizes and focuses windows of other apps. Coordinates are global
// points with a top-left origin. Handles are opaque and valid only in this session.
type WindowManagement interface {
	// Apply lays out the focused window of the app that was frontmost before Nevermind opened.
	Apply(layout WindowLayout, options WindowManagementOptions) (ApplyResult, error)
	ApplyTo(handle WindowHandle, layout WindowLayout, options WindowManagementOptions) (ApplyResult, error)
	Displays() ([]DisplayInfo, error)
	Windows(query WindowQuery) ([]WindowInfo, error)
	// WaitForWindow polls Windows until a match exists or timeout passes (default 5 s, max 30 s).
	WaitForWindow(query WindowQuery, timeout time.Duration) (window WindowInfo, found bool, err error)
	Focus(handle WindowHandle) (FocusResult, error)
	// FocusNeighbor focuses the nearest window in a direction: left, right, up or down.
	FocusNeighbor(direction string, wrap bool) (FocusResult, error)
}

// FindOptions filters a directory walk. Kind may be image, video, media or file. Scanning stops at
// ScanLimit candidates before sorting, so huge roots return approximate newest results.
type FindOptions struct {
	Limit             int      `json:"limit,omitempty"`
	Depth             int      `json:"depth,omitempty"`
	Extensions        []string `json:"extensions,omitempty"`
	Kind              string   `json:"kind,omitempty"`
	Patterns          []string `json:"patterns,omitempty"`
	SortBy            string   `json:"sortBy,omitempty"` // recent, modified, added, created, name or size
	Order             string   `json:"order,omitempty"`  // asc or desc
	IncludeDimensions bool     `json:"includeDimensions,omitempty"`
	ScanLimit         int      `json:"scanLimit,omitempty"`
}

// IndexOptions filters the host file index. Query filters an existing snapshot.
type IndexOptions struct {
	FindOptions
	Roots         []string `json:"roots,omitempty"`
	IncludeHidden bool     `json:"includeHidden,omitempty"`
	Ignore        []string `json:"ignore,omitempty"`
	Query         string   `json:"query,omitempty"`
}

// IndexReport is returned by Files.Reindex.
type IndexReport struct {
	Count int      `json:"count"`
	Roots []string `json:"roots"`
}

// Files finds, reads, writes and previews local files. Paths may start with "~".
type Files interface {
	// Find walks roots. Missing roots are skipped with a logged warning, so one batch covers them all.
	Find(roots []string, options FindOptions) ([]FileRef, error)
	OpenWithApps(path string) ([]App, error)
	Open(path string) error
	Reveal(path string) error
	// QuickLook opens the OS previewer outside Nevermind; check System().HasCapability("quick-look").
	QuickLook(path string) error
	ReadText(path string) (string, error)
	// WriteText fails when the file exists unless overwrite is set. It never creates folders.
	WriteText(path, text string, overwrite bool) (FileRef, error)
	// WriteBytes saves raw bytes, such as a downloaded image. It fails when the file exists unless overwrite is set,
	// never creates folders and replaces the file atomically. The host accepts at most 512 KiB per call.
	WriteBytes(path string, data []byte, overwrite bool) (FileRef, error)
	DisplayPath(path string) string
	FileURL(path string) string
	Thumbnail(path string) (url string, found bool)
	Metadata(path string) (FileRef, error)
	IndexedRoots() []string
	IndexSnapshot(options IndexOptions) []IndexedFile
	Reindex(options IndexOptions) (IndexReport, error)
	Recent(options IndexOptions) []IndexedFile
	SearchIndex(query string, options IndexOptions) []IndexedFile
}

// ShellOptions controls a process. Zero TimeoutMs and OutputLimit take the host defaults: 30 s, clamped to
// 1 s to 120 s, and 200000 characters per stream. Shell makes Exec join the command and its arguments into one
// line for /bin/sh -c; Script always runs under /bin/bash as a login shell.
type ShellOptions struct {
	CWD         string            `json:"cwd,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	TimeoutMs   int64             `json:"timeoutMs,omitempty"`
	OutputLimit int64             `json:"outputLimit,omitempty"`
	Shell       bool              `json:"shell,omitempty"`
}

// ShellResult is the captured output of a process.
type ShellResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// Shell runs processes. Declare the system capability for review; it is metadata, not a sandbox.
type Shell interface {
	OpenExternal(url string) error
	Exec(command string, args []string, options ShellOptions) (ShellResult, error)
	Script(script string, options ShellOptions) (ShellResult, error)
	AppleScript(script string, options ShellOptions) (ShellResult, error)
	Which(command string) (ShellResult, error)
}
