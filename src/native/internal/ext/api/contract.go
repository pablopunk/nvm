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

// ActionHandler runs ActionRun actions, command bodies, form submits, editor saves, refreshes and
// pagination. Dispatch is by Invocation.Ref.ID, so one extension has one switch statement.
type ActionHandler interface {
	RunAction(ctx Context, in Invocation) (Result, error)
}

// DataLoader fills a view whose Source names a loader. It is called in the background; the host
// patches the visible view when it returns.
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

// Context is the single argument of every handler. It is not serializable: it is the host's
// in-process service surface. Every accessor is cheap and never blocks on decoration work.
type Context interface {
	// Std carries cancellation and deadlines. It is cancelled when the query changes, the view
	// closes, or the host call budget (B8: 20 ms soft, 100 ms hard for Search) expires.
	Std() context.Context
	Launch() LaunchContext
	Identity() Identity
	Clock() Clock

	Storage() Storage
	Secrets() Secrets
	Cache() Cache
	Settings() Settings
	Logs() Logs
	Text() Text
	Data() Data
	Drafts() Drafts
	Clipboard() ClipboardHistory
	Desktop() Desktop
	OCR() OCR
	Dictation() Dictation
	Shortcuts() Shortcuts
	Indicators() Indicators
	System() System
	Updates() Updates
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
	// Refresh returns an action that re-runs the visible view's Refresh handler.
	Refresh() Action
	// Invalidate drops the visible view's data source snapshot so the next open is fresh.
	Invalidate()
	// RefreshMenuBar re-renders this extension's menu bar items.
	RefreshMenuBar()
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
