package windowmanagement

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// unavailableMessage is what a window command says where the system cannot manage other apps' windows.
const unavailableMessage = "Window management is not available on this system."

// layoutByID finds a layout command by its ID without the "layout." prefix.
func layoutByID(id string) (layout, bool) {
	for _, l := range layouts {
		if l.id == id {
			return l, true
		}
	}
	return layout{}, false
}

// desktopMoveByID finds the Next or Previous Desktop command by its ID.
func desktopMoveByID(id string) (desktopMove, bool) {
	for _, move := range desktopMoves {
		if move.id == id {
			return move, true
		}
	}
	return desktopMove{}, false
}

// applyLayout places the focused window of the app in front, with the layout the settings size.
func (e Extension) applyLayout(ctx api.Context, l layout) (api.Result, error) {
	settings := LoadSettings(ctx, e.start)
	windows := ctx.Desktop().WindowManagement()
	if windows == nil {
		return toast(unavailableMessage, "error"), nil
	}
	result, err := windows.Apply(l.spec(settings), optionsFor(settings))
	if err != nil {
		return api.Result{}, err
	}
	return layoutResult(result), nil
}

// applyToWindow places one listed window with a layout. The window comes from the Switch Window list.
func (e Extension) applyToWindow(ctx api.Context, handle api.WindowHandle, l layout) (api.Result, error) {
	settings := LoadSettings(ctx, e.start)
	windows := ctx.Desktop().WindowManagement()
	if windows == nil {
		return toast(unavailableMessage, "error"), nil
	}
	result, err := windows.ApplyTo(handle, l.spec(settings), optionsFor(settings))
	if err != nil {
		return api.Result{}, err
	}
	return layoutResult(result), nil
}

// layoutResult says what a layout could not do: a refused placement is an error toast, and a partial placement gives its
// reason. A window that reached its frame says nothing.
func layoutResult(result api.ApplyResult) api.Result {
	switch {
	case !result.OK:
		return toast(result.Message, "error")
	case result.Outcome == "partial" && result.Reason != "":
		return toast(result.Reason, "")
	}
	return api.Result{}
}

// focusNeighborRun raises the nearest window in a direction, and says why it could not.
func focusNeighborRun(direction string) func(api.Context, api.Invocation) (api.Result, error) {
	return func(ctx api.Context, _ api.Invocation) (api.Result, error) {
		windows := ctx.Desktop().WindowManagement()
		if windows == nil {
			return toast(unavailableMessage, "error"), nil
		}
		result, err := windows.FocusNeighbor(direction, false)
		if err != nil {
			return api.Result{}, err
		}
		return focusResult(result), nil
	}
}

// switchDesktop moves to the next or previous desktop, and says why it could not.
func switchDesktop(ctx api.Context, move desktopMove) (api.Result, error) {
	windows := ctx.Desktop().WindowManagement()
	if windows == nil {
		return toast(unavailableMessage, "error"), nil
	}
	result, err := windows.SwitchDesktop(move.direction)
	if err != nil {
		return api.Result{}, err
	}
	return focusResult(result), nil
}

// focusResult reports a focus or desktop move that did not happen. A move that did is silent.
func focusResult(result api.FocusResult) api.Result {
	if !result.OK && result.Message != "" {
		return toast(result.Message, "error")
	}
	return api.Result{}
}

// focusWindow raises one listed window and its app.
func focusWindow(ctx api.Context, handle api.WindowHandle) (api.Result, error) {
	windows := ctx.Desktop().WindowManagement()
	if windows == nil {
		return toast(unavailableMessage, "error"), nil
	}
	result, err := windows.Focus(handle)
	if err != nil {
		return api.Result{}, err
	}
	return focusResult(result), nil
}

func toast(message, tone string) api.Result {
	return api.Result{Toast: &api.Toast{Message: message, Tone: tone}}
}
