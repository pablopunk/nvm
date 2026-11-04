package services

import "github.com/pablopunk/nvm/native/internal/ext/protocol"

// serveViews answers the Views operations an extension may call from its handlers. Only RefreshMenuBar exists: the
// palette's own view operations have no meaning inside a menu bar render.
func (h *Handler) serveViews(call protocol.ServiceCall) (any, error) {
	if call.Op != protocol.OpRefreshMenuBar {
		return nil, invalid("views has no operation %q", call.Op)
	}
	if h.deps.MenuBarRefresh != nil {
		h.deps.MenuBarRefresh(h.manifest.ID)
	}
	return nil, nil
}
