package services

import (
	"context"
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// serveExtensions lists the installed extensions with the extensions declaration and, with extensions.write and from a
// palette action, turns one off, opens its review or shows or hides its menu bar item. No operation enables an
// extension: the review it opens waits for the user. Capabilities are review metadata, so letting any extension open a
// review is safe; the user still reads the capabilities and decides.
func (h *Handler) serveExtensions(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if h.deps.Extensions == nil {
		return nil, unavailable("managing extensions is not available in this build")
	}
	if call.Op == protocol.OpList {
		if err := h.require(api.CapabilityExtensions); err != nil {
			return nil, err
		}
		return h.deps.Extensions.List(), nil
	}
	if err := h.require(api.CapabilityExtensionsWrite); err != nil {
		return nil, err
	}
	if err := requireRun(protocol.RequestMethodOf(ctx), "changing an extension"); err != nil {
		return nil, err
	}
	switch call.Op {
	case protocol.OpDisable:
		id, err := h.otherExtensionID(call.Args)
		if err != nil {
			return nil, err
		}
		return nil, h.deps.Extensions.Disable(id)
	case protocol.OpOpenReview:
		id, err := h.otherExtensionID(call.Args)
		if err != nil {
			return nil, err
		}
		return nil, h.deps.Extensions.OpenReview(id)
	case protocol.OpSetMenuBarItem:
		var args protocol.ExtensionMenuBarArgs
		if err := decodeArgs(call.Args, &args); err != nil {
			return nil, err
		}
		if args.ID == "" {
			return nil, invalid("a menu bar change needs an extension id")
		}
		return nil, h.deps.Extensions.SetMenuBarItem(args.ID, args.Shown)
	}
	return nil, invalid("extensions has no operation %q", call.Op)
}

// otherExtensionID decodes the extension an operation acts on. An extension cannot turn itself off or review itself:
// the user does both in Extensions.
func (h *Handler) otherExtensionID(raw json.RawMessage) (string, error) {
	var args protocol.ExtensionArgs
	if err := decodeArgs(raw, &args); err != nil {
		return "", err
	}
	if args.ID == "" {
		return "", invalid("an extension change needs an extension id")
	}
	if args.ID == h.manifest.ID {
		return "", invalid("an extension cannot change its own state; the user can in Extensions")
	}
	return args.ID, nil
}
