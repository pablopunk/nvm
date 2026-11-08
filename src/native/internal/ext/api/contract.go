package api

import "context"

// Extension is the only method every extension must implement. Manifest runs at load time and
// must be pure: no I/O, no services, no clock. The host reads the manifest before it enables
// anything, so a manifest that depends on state would make review meaningless.
type Extension interface {
	Manifest() Manifest
}

// CommandProvider declares durable commands. The host calls it at load and on refresh, and diffs
// the result: IDs must stay stable across calls because global shortcuts and aliases bind to them.
type CommandProvider interface {
	Commands(ctx Context) ([]Command, error)
}

// RootProvider returns empty-query rows. Keep them small, stable and cheap: the host caches them
// and never waits for them before the palette paints.
type RootProvider interface {
	RootItems(ctx Context) ([]Item, error)
}

// SearchProvider returns query-aware rows. The host debounces, caps, ranks, caches and isolates
// the call. Std() is cancelled when the query changes, so long work must watch it.
type SearchProvider interface {
	SearchItems(ctx Context, query string) ([]Item, error)
}

// ActionHandler runs actions. Return Source-backed views before slow I/O;
// DataLoader fills them after paint. Dispatch uses Invocation.Ref.ID for commands, forms, saves and refreshes.
type ActionHandler interface {
	RunAction(ctx Context, in Invocation) (Result, error)
}

// DataLoader fills Source-backed views after paint. The host shows loading feedback and promotes successful results
// into the Source snapshot; return the view immediately from RunAction and perform slow I/O here.
type DataLoader interface {
	LoadItems(ctx Context, loader string) ([]Item, error)
}

// WindowRestorer rebuilds a persistent window after relaunch from current extension code. Returning
// nil purges the saved restoration state for that key.
type WindowRestorer interface {
	RestoreWindow(ctx Context, restoreKey string) (*WindowRestore, error)
}

// WindowRestore is the fresh view and options returned by RestoreWindow.
type WindowRestore struct {
	View    View           `json:"view"`
	Options *WindowOptions `json:"options,omitempty"`
}

// MenuBarProvider contributes status items. Triggers says when the host re-renders them; the
// renders happen on load, on menu open, on those triggers and on Views.RefreshMenuBar.
type MenuBarProvider interface {
	MenuBarTriggers() []Trigger
	MenuBars(ctx Context) ([]MenuBar, error)
}

// RowActionProvider adds actions to the menu of an installed extension's row, such as Tweak with AI. The host asks each
// built-in that implements it, after the actions it lists itself. Only built-in extensions may implement it.
type RowActionProvider interface {
	RowActions(ctx Context, row ExtensionRow) ([]Action, error)
}

// ExtensionRow names the row a RowActionProvider adds actions to: an extension's own row, with an empty CommandID, or
// one of its commands.
type ExtensionRow struct {
	ExtensionID string `json:"extensionId"`
	CommandID   string `json:"commandId,omitempty"`
	Title       string `json:"title"`
}

// Keys of the Ref.Args that carry an ExtensionRow in a row-actions invocation.
const (
	rowArgExtensionID = "extensionId"
	rowArgCommandID   = "commandId"
	rowArgTitle       = "title"
)

// Args encodes the row as the arguments of the invocation a host sends a RowActionProvider.
func (r ExtensionRow) Args() map[string]string {
	return map[string]string{rowArgExtensionID: r.ExtensionID, rowArgCommandID: r.CommandID, rowArgTitle: r.Title}
}

// RowFromArgs decodes the row a host sent in the arguments of a row-actions invocation.
func RowFromArgs(args map[string]string) ExtensionRow {
	return ExtensionRow{ExtensionID: args[rowArgExtensionID], CommandID: args[rowArgCommandID], Title: args[rowArgTitle]}
}

// Context is the single argument of every handler. It is not serializable: it is the host's
// in-process service surface. Every accessor is cheap and never blocks on decoration work.
type Context interface {
	// Std carries cancellation and deadlines. It is cancelled when the query changes, the view
	// closes, or the host call budget (B8: 20 ms soft, 100 ms hard for Search) expires.
	Std() context.Context
	Launch() LaunchContext
	Identity() Identity
	Self() Self
	Clock() Clock

	Storage() Storage
	Secrets() Secrets
	Cache() Cache
	Settings() Settings
	Logs() Logs
	Text() Text
	Data() Data
	Network() Network
	Drafts() Drafts
	Notes() Notes
	Clipboard() ClipboardHistory
	Desktop() Desktop
	OCR() OCR
	Dictation() Dictation
	Shortcuts() Shortcuts
	Account() Account
	Indicators() Indicators
	Scheduler() Scheduler
	System() System
	Updates() Updates
	Diagnostics() Diagnostics
	Jobs() Jobs
	AI() AI
	AIBuilder() AIBuilder
	Ownership() Ownership
	Windows() Windows
	Navigation() Navigation
	Input() Input
	Views() Views
}

// Views refreshes or invalidates the view that is currently visible.
type Views interface {
	// Refresh returns an action for lists or grids without Search: reload the source or run the Refresh handler. Other views report refresh unavailable.
	Refresh() Action
	// Invalidate drops this installation's data source snapshots so their next open loads fresh data.
	// Already running loads cannot restore invalidated snapshots.
	Invalidate()
	// RefreshMenuBar re-renders this extension's menu bar items.
	RefreshMenuBar()
	// InvalidateRoot runs this extension's root rows again, so rows it changed show without a new query. Other extensions
	// keep their rows. It does nothing while the root is not on top; the next time the root shows, these rows run again.
	// Calls are limited to five at once and 150 a minute per extension; a call over the limit is ignored, so a RootItems
	// that calls it cannot loop.
	InvalidateRoot()
	// RefreshSearch asks the host to search again for query when the palette still shows it; an empty query is the
	// root. The call returns at once. The host shares one search among the requests that arrive together, and does
	// nothing for a query the palette no longer shows, so a background source can call it whenever its data arrives.
	RefreshSearch(query string)
}

// Navigation builds results that change the palette stack. The host implements them so that tests
// can substitute a fake Context; they have no side effects of their own.
type Navigation interface {
	Root(view *View) Result
	Push(view *View) Result
	Replace(view *View) Result
	Pop() Result
	Run(action Action) Result
}

// Input builds a palette prompt that collects short values one field at a time and then runs
// Action with the answers in Invocation.Form.
type Input interface {
	Prompt(spec PromptSpec) Action
}

// PromptSpec describes a palette prompt.
type PromptSpec struct {
	Title       string  `json:"title,omitempty"`
	Message     string  `json:"message,omitempty"`
	Fields      []Field `json:"fields"`
	Action      Action  `json:"action"`
	SubmitTitle string  `json:"submitTitle,omitempty"`
}
