package real

import "github.com/pablopunk/nvm/native/internal/platform"

// paster sends the paste gesture to the frontmost app. No OS implementation exists yet: WP1.6b adds the
// macOS helper, Windows SendInput and the X11 strategy. Until then the port reports ErrUnsupported, and
// the capability clipboard.paste is absent from Capabilities, so the palette omits the paste action.
type paster struct{}

// PasteIntoFrontmost reports that paste is not implemented on this build yet.
func (paster) PasteIntoFrontmost(string) error {
	return platform.ErrUnsupported
}
