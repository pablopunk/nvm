package services

import (
	"errors"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/api/budget"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// Per-extension rates of the desktop services, as the extension API states them in package budget. Opening, launching,
// window moves and clipboard writes share one bucket; OCR and screen reads have their own, because one screen read holds
// a worker for seconds.
const (
	desktopActionsPerMinute = budget.DesktopActionsPerMinute
	processRunsPerMinute    = budget.ShellRunsPerMinute
	ocrReadsPerMinute       = budget.OCRReadsPerMinute
	screenReadsPerMinute    = budget.ScreenReadsPerMinute
	notificationsPerMinute  = budget.NotificationsPerMinute
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

// processBudgetExceeded is the quota refusal of the process limiter. It names the budget and says what to do instead,
// because an extension that scans many items meets it as a surprise.
func processBudgetExceeded() error {
	return quota(fmt.Errorf("process runs are rate limited to %d per minute; run one script that handles many items, or wait a minute and try again", processRunsPerMinute))
}
