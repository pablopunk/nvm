package platform

import "context"

// DesktopSwitcher sends one workspace-navigation intent. Supported is structural support,
// independent of current Automation permission or the user's Mission Control shortcuts.
// A successful dispatch does not prove that the desktop changed.
type DesktopSwitcher interface {
	Supported() bool
	Switch(ctx context.Context, direction string) (HelperResult, error)
}
