//go:build darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// newWindowManager returns the macOS window backend: Accessibility, reached through the bundled window helper.
func newWindowManager(helper platform.Helper) platform.WindowManager {
	return macWindows{macHelper: newMacHelper(helper)}
}
