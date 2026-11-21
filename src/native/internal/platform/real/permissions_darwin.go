//go:build darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// newPermissions returns the macOS privacy port: Accessibility through the window helper, the microphone through the
// helper's microphone commands.
func newPermissions(helper platform.Helper) platform.Permissions {
	return macPermissions{access: newMacHelper(helper), mic: macMicrophone{helper: helper, binary: macosHelperPath()}}
}
