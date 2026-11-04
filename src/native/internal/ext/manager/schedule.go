package manager

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// runScheduled runs a one-shot run the extension scheduled when it comes due. It runs through Run, as a palette action
// does. Nothing waits for the result, so a run that fails is logged.
func (m *Manager) runScheduled(extID string, inv api.Invocation) {
	_, failure, err := m.Run(context.Background(), extID, inv)
	if err == nil && failure == nil {
		return
	}
	fields := logx.F{"extension": extID, "action": inv.Ref.ID}
	if err != nil {
		fields["error"] = err.Error()
	}
	if failure != nil {
		fields["message"] = failure.Message
	}
	logx.Default().Warn("scheduled run failed", fields)
}
