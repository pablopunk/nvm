package services

import (
	"context"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// SystemCommandAction preserves declarative policy: host-confirmed UI actions require no new manifest grant or quota.
func (h *Handler) SystemCommandAction(ctx context.Context, op, pane string) error {
	id, known := systemfeat.CommandForOp(op)
	if !known || !h.deps.SystemCommands.Supported(id) {
		return platform.ErrUnsupported
	}
	if pane != "" {
		if id != systemfeat.CommandSystemPanes {
			return invalid("only system settings accepts a pane")
		}
		knownPane := false
		for _, offered := range h.settingsPanes() {
			if offered.ID == pane {
				knownPane = true
				break
			}
		}
		if !knownPane {
			return unavailable("this settings pane is not available on this system")
		}
	}
	if err := h.admitDesktopEffect(ctx); err != nil {
		return err
	}
	if pane != "" {
		return h.deps.SystemCommands.OpenPane(systemfeat.Pane{ID: pane}, "")
	}
	return h.deps.SystemCommands.RunContext(ctx, id)
}
