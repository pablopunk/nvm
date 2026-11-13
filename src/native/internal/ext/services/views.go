package services

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// serveViews answers the Views operations an extension may call from its handlers. RefreshMenuBar, InvalidateRoot and
// RefreshSearch exist; the palette's visible-view operations have no meaning inside a menu bar render or a root call.
// A root refresh over the burst limit is ignored, as 1.x ignores it, so a RootItems that refreshes itself cannot loop.
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
	case protocol.OpInvalidateViews:
		if h.deps.ViewsInvalidate != nil {
			h.deps.ViewsInvalidate(h.manifest.ID)
		}
		return nil, nil
	case protocol.OpRefreshSearch:
		return nil, h.refreshSearch(call.Args)
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

// refreshSearch hands a query to the host's search refresh. Without one the request is a no-op.
func (h *Handler) refreshSearch(args json.RawMessage) error {
	var request protocol.SearchRefreshArgs
	if err := decodeArgs(args, &request); err != nil {
		return err
	}
	if h.deps.SearchRefresh != nil {
		h.deps.SearchRefresh(h.manifest.ID, request.Query)
	}
	return nil
}
