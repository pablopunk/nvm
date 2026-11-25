package services

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// serveShortcuts reads the shortcut table and, through the editor, changes the user's shortcuts.
func (h *Handler) serveShortcuts(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityShortcuts); err != nil {
		return nil, err
	}
	switch call.Op {
	case protocol.OpBindable, protocol.OpSet, protocol.OpRemove:
		return h.editShortcuts(ctx, call)
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

// editShortcuts lists the bindable commands, or sets or removes a shortcut after the shortcuts.write check.
func (h *Handler) editShortcuts(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if h.deps.ShortcutEditor == nil {
		return nil, unavailable("changing shortcuts is not available in this build")
	}
	if call.Op == protocol.OpBindable {
		return h.deps.ShortcutEditor.Bindable(), nil
	}
	if err := h.require(api.CapabilityShortcutsWrite); err != nil {
		return nil, err
	}
	if err := requireRun(protocol.RequestMethodOf(ctx), "changing a shortcut"); err != nil {
		return nil, err
	}
	var args protocol.ShortcutArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return nil, err
	}
	if args.ActionID == "" {
		return nil, invalid("a shortcut change needs an action id")
	}
	if call.Op == protocol.OpRemove {
		return nil, h.deps.ShortcutEditor.Remove(ctx, args.ActionID, args.Expect)
	}
	if args.Accelerator == "" {
		return nil, invalid("setting a shortcut needs an accelerator, such as Command+Alt+K")
	}
	return h.deps.ShortcutEditor.Set(ctx, args.ActionID, args.Accelerator, args.Expect)
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
