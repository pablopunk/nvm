package services

import (
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// serveViews answers the Views operations an extension may call from its handlers. RefreshMenuBar and InvalidateRoot
// exist; the palette's visible-view operations have no meaning inside a menu bar render or a root call. A root refresh
// over the burst limit is ignored, as 1.x ignores it, so a RootItems that refreshes itself cannot loop.
func (h *Handler) serveViews(call protocol.ServiceCall) (any, error) {
	switch call.Op {
	case protocol.OpRefreshMenuBar:
		if h.deps.MenuBarRefresh != nil {
			h.deps.MenuBarRefresh(h.manifest.ID)
		}
		return nil, nil
	case protocol.OpInvalidateRoot:
		h.refreshRoot()
		return nil, nil
	}
	return nil, invalid("views has no operation %q", call.Op)
}

// refreshRoot asks the host to run the root's rows again, unless this extension has refreshed too often.
func (h *Handler) refreshRoot() {
	if !h.rootRefreshes.allow() {
		h.logger.Warn("extension.refresh.throttled", logx.F{"limit": rootRefreshBurst})
		return
	}
	if h.deps.RootRefresh != nil {
		h.deps.RootRefresh(h.manifest.ID)
	}
}
