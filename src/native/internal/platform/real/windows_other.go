//go:build !linux && !darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// newWindowManager is nil until a Windows backend exists (WP4.1 L6 work). The capability is omitted with it,
// rather than shown disabled.
func newWindowManager(platform.Helper) platform.WindowManager {
	return nil
}
