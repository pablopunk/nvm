package services

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// serveExtension answers what an extension asks about itself. The target is always the calling extension.
func (h *Handler) serveExtension(call protocol.ServiceCall) (any, error) {
	if call.Op != protocol.OpRename {
		return nil, invalid("extension has no operation %q", call.Op)
	}
	var change api.ExtensionRename
	if err := decodeArgs(call.Args, &change); err != nil {
		return nil, err
	}
	if h.deps.Rename == nil {
		return nil, unavailable("extensions cannot be renamed in this build")
	}
	return nil, h.deps.Rename(h.manifest.ID, change)
}
