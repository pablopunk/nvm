package services

import (
	"fmt"
	"github.com/pablopunk/nvm/native/internal/ext/api"
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
