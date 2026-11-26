package services

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// editAliases adds or removes an alias after the shortcuts.write check, from a palette run or a conversation program.
func (h *Handler) editAliases(ctx context.Context, call protocol.ServiceCall) error {
	if h.deps.AliasEditor == nil {
		return unavailable("changing aliases is not available in this build")
	}
	if err := h.require(api.CapabilityShortcutsWrite); err != nil {
		return err
	}
	if err := requireRun(protocol.RequestMethodOf(ctx), "changing an alias"); err != nil {
		return err
	}
	var args protocol.AliasArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return err
	}
	if args.Alias == "" {
		return invalid("an alias change needs the alias")
	}
	if call.Op == protocol.OpRemoveAlias {
		return h.deps.AliasEditor.RemoveAlias(args.Alias)
	}
	if args.ActionID == "" {
		return invalid("adding an alias needs an action id")
	}
	return h.deps.AliasEditor.AddAlias(args.ActionID, args.Alias)
}
