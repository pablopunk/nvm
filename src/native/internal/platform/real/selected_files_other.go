//go:build !darwin

package real

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/platform"
)

type unsupportedSelectedFiles struct{}

// NewSelectedFiles reports the capability unavailable on this platform.
func NewSelectedFiles(platform.Helper) platform.SelectedFiles { return unsupportedSelectedFiles{} }

func (unsupportedSelectedFiles) Paths(context.Context, platform.FocusTarget) ([]string, error) {
	return nil, platform.ErrUnsupported
}
