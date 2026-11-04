//go:build !linux && !darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// windowManagementSupported is false until a Windows backend exists (WP4.1 L6 work); the capability is then
// omitted rather than shown disabled.
func windowManagementSupported() bool {
	return false
}

func newWindowManager(platform.Helper) platform.WindowManager {
	return nil
}
