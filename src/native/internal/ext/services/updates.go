package services

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// UpdateSurface is the host's side of the app updates: the state its updater reports. The actions that change it are
// ActionUpdate actions the host performs on the palette, so the surface only reads.
type UpdateSurface interface {
	State() api.UpdateState
}

// serveUpdates answers the state of the app's updater. Without a surface the host has no updater to report.
func (h *Handler) serveUpdates(call protocol.ServiceCall) (any, error) {
	if h.deps.Updates == nil {
		return nil, unavailable("updates are not available in this build")
	}
	if call.Op != protocol.OpState {
		return nil, invalid("updates has no operation %q", call.Op)
	}
	return h.deps.Updates.State(), nil
}
