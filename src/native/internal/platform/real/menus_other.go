//go:build !darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// NewCurrentAppMenus omits the capability on systems without a menu backend.
func NewCurrentAppMenus(platform.Helper) MenuSourceCapture { return nil }
