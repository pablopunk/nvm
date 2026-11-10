package services

import (
	"context"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// SystemCommandAction preserves declarative policy: host-confirmed UI actions require no new manifest grant or quota.
func (h *Handler) SystemCommandAction(ctx context.Context, op string) error {
	id, known := systemfeat.CommandForOp(op)
	if !known || !h.deps.SystemCommands.Supported(id) {
		return platform.ErrUnsupported
	}
	if err := h.admitDesktopEffect(ctx); err != nil {
		return err
	}
	return h.deps.SystemCommands.RunContext(ctx, id)
}
