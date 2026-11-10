package services

import (
	"context"
	"fmt"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// require refuses a call whose capability the manifest does not declare, and records the refusal.
func (h *Handler) require(capability api.Capability) error {
	if h.capabilities[capability] {
		return nil
	}
	h.logger.Warn("refused a call that needs an undeclared capability", logx.F{"capability": string(capability)})
	return serviceError{
		failure: api.Failure{Code: api.CodePermission, Message: fmt.Sprintf("this extension did not declare the %s capability in its manifest", capability)},
		cause:   api.ErrPermission,
	}
}

// admitDesktopEffect is the final admission boundary for migrated synchronous desktop operations.
// A native operation admitted here may finish after cancellation; it is never replayed.
func (h *Handler) admitDesktopEffect(ctx context.Context) error {
	if h.closed.Load() {
		return errHandlerClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return protocol.LifetimeOf(ctx).Err()
}

func permissionDenied(message string) error {
	return serviceError{failure: api.Failure{Code: api.CodePermission, Message: message}, cause: api.ErrPermission}
}
