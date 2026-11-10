package real

import (
	"context"
	"os"
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

// FrontmostAppID names the app with the keyboard in the form PasteIntoFrontmost matches on this OS.
func (p paster) FrontmostAppID() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), pasteTimeout)
	defer cancel()
	return frontmostAppID(ctx, p.helper, os.Getpid())
}

// UndoInFrontmost waits for the palette to hide, then sends the undo gesture. Like PasteIntoFrontmost, it never sends
// the key to a window it cannot name when expectedAppID is set.
func (p paster) UndoInFrontmost(expectedAppID string) error {
	return p.UndoInFrontmostContext(context.Background(), expectedAppID)
}

// UndoInFrontmostContext waits for the palette to hide, then sends undo using the caller's lifecycle.
func (p paster) UndoInFrontmostContext(parent context.Context, expectedAppID string) error {
	ctx, cancel := context.WithTimeout(parent, pasteTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.waitHidden != nil {
		if err := p.waitHidden(ctx); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return undoGesture(ctx, p.helper, expectedAppID)
}

// PasteIntoFrontmost waits for the palette to hide, then sends the paste gesture. It returns
// platform.ErrUnsupported when the OS or session cannot send it, and never types into a window it cannot
// name when expectedAppID is set.
func (p paster) PasteIntoFrontmost(expectedAppID string) error {
	return p.PasteIntoFrontmostContext(context.Background(), expectedAppID)
}

// PasteIntoFrontmostContext waits for the palette to hide, then sends paste using the caller's lifecycle.
func (p paster) PasteIntoFrontmostContext(parent context.Context, expectedAppID string) error {
	ctx, cancel := context.WithTimeout(parent, pasteTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.waitHidden != nil {
		if err := p.waitHidden(ctx); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return pasteGesture(ctx, p.helper, expectedAppID)
}

// TypeIntoFrontmost waits for the palette to hide, then types text into the frontmost app. It follows the
// PasteIntoFrontmost rules for expectedAppID and returns platform.ErrUnsupported where typing is not offered.
func (p paster) TypeIntoFrontmost(expectedAppID, text string, delay time.Duration) error {
	return p.TypeIntoFrontmostContext(context.Background(), expectedAppID, text, delay)
}

// TypeIntoFrontmostContext types using the caller's lifecycle.
func (p paster) TypeIntoFrontmostContext(parent context.Context, expectedAppID, text string, delay time.Duration) error {
	ctx, cancel := context.WithTimeout(parent, pasteTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.waitHidden != nil {
		if err := p.waitHidden(ctx); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return typeGesture(ctx, p.helper, expectedAppID, text, delay)
}

var _ platform.Typist = paster{}
var _ platform.ContextPaster = paster{}
var _ platform.ContextTypist = paster{}
