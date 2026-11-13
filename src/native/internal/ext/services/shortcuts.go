package services

import (
	"encoding/json"

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
	case protocol.OpAliases:
		return h.shortcutAliases(call.Args)
	}
	return nil, invalid("shortcuts has no operation %q", call.Op)
}

// shortcutAliases reads the aliases of the actions under a prefix. The prefix is required, so a read never lists every
// action the user named.
func (h *Handler) shortcutAliases(raw json.RawMessage) (map[string][]string, error) {
	var args protocol.PrefixArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	if args.Prefix == "" {
		return nil, invalid("an alias read needs an action prefix")
	}
	return h.deps.Shortcuts.Aliases(args.Prefix), nil
}
