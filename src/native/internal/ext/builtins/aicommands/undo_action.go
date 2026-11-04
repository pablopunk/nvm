package aicommands

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// errNothingToUndo is the failure of an undo with no fix to put back.
var errNothingToUndo = errors.New("There is no fix to undo")

// undo sends the undo gesture to the app the palette was summoned from. The host refuses when the window in front is
// not the one the fix landed in.
func (e Extension) undo(ctx api.Context) error {
	hud := newHUD(ctx, UndoTitle)
	if !e.fixes.take(ctx.Clock().Now()) {
		return hud.fail(errNothingToUndo)
	}
	if err := ctx.Desktop().Keyboard().Undo(); err != nil {
		return hud.fail(err)
	}
	return hud.succeed("The text is back as it was", undoDismissMs)
}
