package services

import (
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// serveAccount answers the account reads. Signing in is not a read: it is the ActionAccount action, which the host runs.
func (h *Handler) serveAccount(call protocol.ServiceCall) (any, error) {
	if h.deps.Account == nil {
		return nil, unavailable("the account is not available in this build")
	}
	if call.Op != protocol.OpSignedIn {
		return nil, invalid("account has no operation %q", call.Op)
	}
	return h.deps.Account.SignedIn(), nil
}
