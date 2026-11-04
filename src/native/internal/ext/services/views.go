package services

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// serveViews answers the Views operations an extension may call from its handlers. RefreshMenuBar re-renders the
// menu bar items; RefreshSearch asks the palette to search again for a query. Both return at once.
func (h *Handler) serveViews(call protocol.ServiceCall) (any, error) {
	switch call.Op {
	case protocol.OpRefreshMenuBar:
		if h.deps.MenuBarRefresh != nil {
			h.deps.MenuBarRefresh(h.manifest.ID)
		}
		return nil, nil
	case protocol.OpRefreshSearch:
		return nil, h.refreshSearch(call.Args)
	}
	return nil, invalid("views has no operation %q", call.Op)
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
