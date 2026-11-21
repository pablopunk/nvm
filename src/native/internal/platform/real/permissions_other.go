//go:build !darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// newPermissions returns the privacy port of this OS. Only macOS can read or request a permission so far; elsewhere
// every permission stays unknown, and its settings view opens the matching pane.
func newPermissions(platform.Helper) platform.Permissions {
	return permissions{}
}
