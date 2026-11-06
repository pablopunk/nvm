package services

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// MenuPort is bound by the host to the current source and owns handle validation.
type MenuPort interface {
	Snapshot(context.Context) (api.MenuSnapshot, error)
	Invoke(context.Context, string) error
}

func (h *Handler) serveDesktopMenus(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityDesktopMenus); err != nil {
		return nil, err
	}
	if h.deps.Menus == nil {
		return nil, unavailable("menus are not available in this build")
	}
	switch call.Op {
	case protocol.OpSnapshot:
		if len(call.Args) != 0 && string(call.Args) != "null" && string(call.Args) != "{}" {
			return nil, invalid("snapshot takes no arguments")
		}
		return h.deps.Menus.Snapshot(ctx)
	case protocol.OpInvoke:
		var args protocol.HandleArgs
		dec := json.NewDecoder(bytes.NewReader(call.Args))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&args); err != nil {
			return nil, invalid("invalid menu handle arguments")
		}
		if err := dec.Decode(&struct{}{}); err != io.EOF {
			return nil, invalid("invalid menu handle arguments")
		}
		if strings.TrimSpace(args.Handle) == "" {
			return nil, invalid("menu handle is required")
		}
		return nil, h.deps.Menus.Invoke(ctx, args.Handle)
	}
	return nil, invalid("desktop.menus has no operation %q", call.Op)
}
