package services

import (
	"errors"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// Per-extension rates of the desktop services. Opening, launching, window moves and clipboard writes share one
// bucket; OCR and screen reads have their own, because one screen read holds a worker for seconds.
const (
	desktopActionsPerMinute = 120
	ocrReadsPerMinute       = 20
	screenReadsPerMinute    = 6
)

// requireRun refuses a destructive call that does not run under a palette action. A search or a root listing has
// no confirmation step, so it cannot remove the user's data.
func requireRun(ctxMethod, what string) error {
	if ctxMethod == protocol.MethodRun {
		return nil
	}
	return serviceError{
		failure: api.Failure{Code: api.CodePermission, Message: what + " runs only from a palette action the user confirmed"},
		cause:   api.ErrPermission,
	}
}

// rateExceeded is the quota refusal of a desktop limiter.
func rateExceeded(what string) error {
	return quota(errors.New(what + " is rate limited; wait a minute and try again"))
}
