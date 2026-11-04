package real

import (
	"errors"
	"fmt"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// hotkeys binds global shortcuts through MyGo.
type hotkeys struct{}

// Register binds the accelerator. MyGo reports a shortcut another program holds, and one it cannot parse,
// with an error of its own; both come back as platform.ErrConflict. An unsupported session stays
// ErrUnsupported, so the caller can say so instead of blaming the shortcut.
func (hotkeys) Register(accelerator string, fn func()) error {
	err := mygo.GlobalShortcut.Register(accelerator, fn)
	if err == nil || errors.Is(err, platform.ErrUnsupported) {
		return err
	}
	return fmt.Errorf("%w: %v", platform.ErrConflict, err)
}

// Unregister removes the binding.
func (hotkeys) Unregister(accelerator string) {
	mygo.GlobalShortcut.Unregister(accelerator)
}
