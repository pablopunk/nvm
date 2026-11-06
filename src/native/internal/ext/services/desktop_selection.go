package services

import (
	"bytes"
	"context"
	"encoding/json"
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

type selectionFilesEngine interface {
	Files(context.Context) ([]string, error)
}

func (h *Handler) serveDesktopSelection(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if call.Op == protocol.OpFiles {
		if err := h.require(api.CapabilityDesktopFiles); err != nil {
			return nil, err
		}
		if len(call.Args) != 0 && !bytes.Equal(bytes.TrimSpace(call.Args), []byte("null")) {
			return nil, invalid("selection.files takes no arguments")
		}
		engine, ok := h.deps.Selection.(selectionFilesEngine)
		if !ok {
			return nil, unavailable("selected files are not available in this build")
		}
		paths, err := engine.Files(ctx)
		if errors.Is(err, platform.ErrDenied) {
			return nil, unavailable("Allow Nevermind to access Finder in System Settings.")
		}
		if errors.Is(err, platform.ErrUnsupported) {
			return nil, unavailable("Selected files are not available on this system yet.")
		}
		return paths, err
	}
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
	switch call.Op {
	case protocol.OpUndo:
		return nil, h.undoSelection()
	case protocol.OpTypeText:
		return nil, h.typeTextCall(call.Args)
	}
	return nil, invalid("desktop.keyboard has no operation %q", call.Op)
}

func (h *Handler) undoSelection() error {
	if h.deps.Selection == nil {
		return unavailable("the keyboard is not available in this build")
	}
	if !h.actions.allow() {
		return rateExceeded("desktop actions")
	}
	return selectionFailure(h.deps.Selection.Undo())
}

func (h *Handler) typeTextCall(raw json.RawMessage) error {
	var args protocol.TypeTextArgs
	if err := decodeArgs(raw, &args); err != nil {
		return err
	}
	return h.typeText(args.Text, args.Options.DelayMs)
}
