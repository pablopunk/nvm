//go:build darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// NewCurrentAppMenus provides the macOS menu capability using the bundled helper.
func NewCurrentAppMenus(helper platform.Helper) MenuSourceCapture {
	return macMenus{macHelper: newMacHelper(helper)}
}
