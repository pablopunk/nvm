//go:build darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// newPermissions returns the macOS privacy port, which reads and requests Accessibility through the window helper.
func newPermissions(helper platform.Helper) platform.Permissions {
	return macPermissions{access: newMacHelper(helper)}
}
