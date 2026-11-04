package services

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/features/selection"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// SelectionEngine reads, replaces and undoes the selection of the app the palette was summoned from. The host's
// selection engine satisfies it; a build without one answers unavailable.
type SelectionEngine interface {
	Text() (string, bool, error)
	Replace(text string) (bool, error)
	Undo() error
}

// maxReplaceText bounds one replacement from an extension. A selection larger than this is refused rather than pasted
// in part.
const maxReplaceText = 1 << 20

func (h *Handler) serveDesktopSelection(call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityDesktopSelection); err != nil {
		return nil, err
	}
	if h.deps.Selection == nil {
		return nil, unavailable("the selection is not available in this build")
	}
	switch call.Op {
	case protocol.OpText:
		text, found, err := h.deps.Selection.Text()
		if err != nil {
			return nil, selectionFailure(err)
		}
		return protocol.SelectionTextResult{Text: text, Found: found}, nil
	case protocol.OpReplace:
		return h.replaceSelection(call)
	}
	return nil, invalid("desktop.selection has no operation %q", call.Op)
}

func (h *Handler) replaceSelection(call protocol.ServiceCall) (bool, error) {
	var args protocol.ReplaceTextArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return false, err
	}
	if len(args.Text) > maxReplaceText {
		return false, invalid("the replacement text is longer than %d bytes", maxReplaceText)
	}
	if !h.actions.allow() {
		return false, rateExceeded("desktop actions")
	}
	replaced, err := h.deps.Selection.Replace(args.Text)
	return replaced, selectionFailure(err)
}

// selectionFailure says a selection or keyboard failure in the words the palette shows. The host's own failures carry
// their copy; a platform refusal is named by what the user can do about it.
func selectionFailure(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, platform.ErrConflict):
		return unavailable("The focused app changed, so nothing was pasted.")
	case errors.Is(err, platform.ErrDenied):
		return unavailable("Allow Nevermind to control the keyboard in System Settings.")
	case errors.Is(err, platform.ErrUnsupported):
		return unavailable("Selected text is not available on this system yet.")
	case errors.Is(err, selection.ErrSourceGone), errors.Is(err, selection.ErrFormattedClipboard),
		errors.Is(err, selection.ErrKeyboardHeld), errors.Is(err, selection.ErrNothingToUndo),
		errors.Is(err, selection.ErrSourceChanged):
		return unavailable("%s", err.Error())
	}
	return err
}

func (h *Handler) serveDesktopKeyboard(call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityDesktopKeyboard); err != nil {
		return nil, err
	}
	if h.deps.Selection == nil {
		return nil, unavailable("the keyboard is not available in this build")
	}
	if call.Op != protocol.OpUndo {
		return nil, invalid("desktop.keyboard has no operation %q", call.Op)
	}
	if !h.actions.allow() {
		return nil, rateExceeded("desktop actions")
	}
	return nil, selectionFailure(h.deps.Selection.Undo())
}
