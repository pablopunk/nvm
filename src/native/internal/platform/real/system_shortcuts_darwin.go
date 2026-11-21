//go:build darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

func newSystemShortcuts(helper platform.Helper) platform.SystemShortcuts {
	return symbolicHotkeys{helper: helper}
}
