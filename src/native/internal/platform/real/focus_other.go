//go:build !linux

package real

import "github.com/pablopunk/nvm/native/internal/platform"

// newFocus returns the focus port of this OS. Off Linux the palette's focus is not returned by the app: the
// capability is omitted, so the port reports ErrUnsupported.
func newFocus() platform.Focus { return unsupportedFocus{} }

type unsupportedFocus struct{}

func (unsupportedFocus) Active() (platform.FocusTarget, error) {
	return platform.FocusTarget{}, platform.ErrUnsupported
}

func (unsupportedFocus) Restore(platform.FocusTarget) error { return platform.ErrUnsupported }
