//go:build !darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// newSystemShortcuts reports that this platform has no live list of the shortcuts the OS uses: Linux and Windows
// keep theirs in desktop-specific settings with no one reliable reader.
func newSystemShortcuts(platform.Helper) platform.SystemShortcuts { return nil }
