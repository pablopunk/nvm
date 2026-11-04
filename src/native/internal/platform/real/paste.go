package real

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// pasteTimeout bounds the wait for the palette to hide plus the key gesture.
const pasteTimeout = 3 * time.Second

// paster sends the paste gesture to the frontmost app. The gesture itself is per OS (paster_*.go). The
// composition root supplies waitHidden, which returns once the palette window has hidden and focus has gone
// back to the app that had it; a nil waitHidden sends the gesture at once.
type paster struct {
	helper     platform.Helper
	waitHidden func(ctx context.Context) error
}

// NewPaster returns the production paster over helper. waitHidden is called before the gesture and must return
// once the palette window has hidden and focus has gone back to the previous app; the composition root supplies
// it, so the paster never polls the window.
func NewPaster(helper platform.Helper, waitHidden func(ctx context.Context) error) platform.Paster {
	return paster{helper: helper, waitHidden: waitHidden}
}

// PasteIntoFrontmost waits for the palette to hide, then sends the paste gesture. It returns
// platform.ErrUnsupported when the OS or session cannot send it, and never types into a window it cannot
// name when expectedAppID is set.
func (p paster) PasteIntoFrontmost(expectedAppID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), pasteTimeout)
	defer cancel()
	if p.waitHidden != nil {
		if err := p.waitHidden(ctx); err != nil {
			return err
		}
	}
	return pasteGesture(ctx, p.helper, expectedAppID)
}
