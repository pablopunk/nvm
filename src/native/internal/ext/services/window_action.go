package services

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// ViewWindowActions is the host-owned executor for declarative view windows, used on the UI thread.
// It is distinct from desktop window management and is never supplied by an extension.
type ViewWindowActions interface {
	Perform(owner string, action api.Action) error
}

// WindowAction preserves declarative UI policy without adding a manifest grant.
// The palette may have dismissed; installation/host admission still applies before the synchronous UI effect.
func (h *Handler) WindowAction(ctx context.Context, action api.Action, windows ViewWindowActions) error {
	if action.Kind != api.ActionWindow {
		return invalid("a window action is required")
	}
	if err := action.Validate(); err != nil {
		return invalid("%s", err)
	}
	if windows == nil {
		return unavailable("extension windows are not available in this build")
	}
	if err := h.admitDesktopEffect(ctx); err != nil {
		return err
	}
	return windows.Perform(h.manifest.ID, action)
}
