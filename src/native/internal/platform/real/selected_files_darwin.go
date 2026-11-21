//go:build darwin

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// NewSelectedFiles provides the selected-files capability on macOS.
func NewSelectedFiles(helper platform.Helper) platform.SelectedFiles {
	return selectedFilesReader{helper: helper}
}
