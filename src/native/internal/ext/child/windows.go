package child

import "github.com/pablopunk/nvm/native/internal/ext/api"

// childWindows builds the window actions an extension returns. The host performs them, so building one is data only.
type childWindows struct{}

func (childWindows) Create(view api.View, options api.WindowOptions) api.Action {
	return windowAction("create", api.WindowRef{View: &view, Options: &options})
}

func (childWindows) Show(ref api.WindowRef) api.Action {
	return windowAction("show", ref)
}

func (childWindows) Hide(ref api.WindowRef) api.Action {
	return windowAction("hide", ref)
}

func (childWindows) Toggle(ref api.WindowRef) api.Action {
	return windowAction("toggle", ref)
}

func (childWindows) Close(ref api.WindowRef) api.Action {
	return windowAction("close", ref)
}

func windowAction(op string, ref api.WindowRef) api.Action {
	return api.Action{Kind: api.ActionWindow, Op: op, Window: &ref}
}
