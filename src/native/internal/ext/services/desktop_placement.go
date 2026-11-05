package services

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// serveDesktopWindows lists displays and windows, places them and moves focus through the window engine. Handles come
// from a listing and expire; the engine refuses a stale one. Layouts the engine cannot read come back as results.
func (h *Handler) serveDesktopWindows(ctx context.Context, call protocol.ServiceCall) (any, error) {
	if err := h.require(api.CapabilityWindowManagement); err != nil {
		return nil, err
	}
	if h.deps.Windows == nil {
		return nil, unavailable("window management is not available on this system")
	}
	switch call.Op {
	case protocol.OpDisplays:
		return h.deps.Windows.Displays(ctx)
	case protocol.OpWindows:
		return h.listWindows(ctx, call)
	case protocol.OpFocus:
		return h.focusWindow(ctx, call)
	case protocol.OpApplyTo:
		return h.applyToWindow(ctx, call)
	case protocol.OpApply:
		return h.applyFrontmost(ctx, call)
	case protocol.OpFocusNeighbor:
		return h.focusNeighbor(ctx, call)
	}
	return nil, invalid("desktop.windows has no operation %q", call.Op)
}

func (h *Handler) listWindows(ctx context.Context, call protocol.ServiceCall) ([]api.WindowInfo, error) {
	var query api.WindowQuery
	if err := decodeArgs(call.Args, &query); err != nil {
		return nil, err
	}
	return h.deps.Windows.Windows(ctx, query)
}

func (h *Handler) focusWindow(ctx context.Context, call protocol.ServiceCall) (api.FocusResult, error) {
	var args protocol.FocusArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return api.FocusResult{}, err
	}
	if !h.actions.allow() {
		return api.FocusResult{}, rateExceeded("window moves")
	}
	return h.deps.Windows.Focus(ctx, args.Handle)
}

func (h *Handler) applyToWindow(ctx context.Context, call protocol.ServiceCall) (api.ApplyResult, error) {
	var args protocol.ApplyToArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return api.ApplyResult{}, err
	}
	if err := args.Layout.Validate(); err != nil {
		return api.ApplyResult{}, invalid("%s", err.Error())
	}
	if !h.actions.allow() {
		return api.ApplyResult{}, rateExceeded("window moves")
	}
	return h.deps.Windows.ApplyTo(ctx, args.Handle, args.Layout, args.Options)
}

func (h *Handler) applyFrontmost(ctx context.Context, call protocol.ServiceCall) (api.ApplyResult, error) {
	var args protocol.ApplyArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return api.ApplyResult{}, err
	}
	if err := args.Layout.Validate(); err != nil {
		return api.ApplyResult{}, invalid("%s", err.Error())
	}
	if !h.actions.allow() {
		return api.ApplyResult{}, rateExceeded("window moves")
	}
	return h.deps.Windows.Apply(ctx, args.Layout, args.Options)
}

func (h *Handler) focusNeighbor(ctx context.Context, call protocol.ServiceCall) (api.FocusResult, error) {
	var args protocol.FocusNeighborArgs
	if err := decodeArgs(call.Args, &args); err != nil {
		return api.FocusResult{}, err
	}
	if !h.actions.allow() {
		return api.FocusResult{}, rateExceeded("window moves")
	}
	return h.deps.Windows.FocusNeighbor(ctx, args.Direction, args.Wrap)
}
