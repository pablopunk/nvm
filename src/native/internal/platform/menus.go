package platform

import "context"

// MenuSource is captured before the palette takes focus. Identity is an OS-owned,
// non-reusable process identity; WindowKey is optional where the OS can verify it.
type MenuSource struct {
	PID       int
	Identity  string
	WindowKey string
}

// MenuEntry is an adapter-owned reference and display metadata. Reference never
// crosses the host boundary; the adapter must revalidate it before invocation.
type MenuEntry struct {
	Reference string
	Title     string
	Path      []string
	Shortcut  string
	Enabled   bool
	Checked   bool
}

// CurrentAppMenus reads and invokes menus for an explicitly captured source.
// Implementations must check access without prompting and validate source identity,
// menu hierarchy and enabled state immediately before invoking. Unsupported OSes
// provide no implementation. Calls must honor context cancellation.
type CurrentAppMenus interface {
	Snapshot(context.Context, MenuSource) ([]MenuEntry, error)
	Invoke(context.Context, MenuSource, string) error
}
