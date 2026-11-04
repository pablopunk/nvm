package services

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// serveShortcuts reads the shortcut table. Registering shortcuts is a separate path through the shortcuts manager.
func (h *Handler) serveShortcuts(call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityShortcuts); err != nil {
		return nil, err
	}
	if h.deps.Shortcuts == nil {
		return nil, unavailable("shortcuts are not available in this build")
	}
	switch call.Op {
	case protocol.OpList:
		return h.deps.Shortcuts.List(), nil
	case protocol.OpPalette:
		return h.deps.Shortcuts.Palette(), nil
	}
	return nil, invalid("shortcuts has no operation %q", call.Op)
}
