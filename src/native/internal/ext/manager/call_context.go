package manager

import (
	"context"

	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

// callContext leaves supervised execution budgets with the child runtime, which excludes bounded host waits.
// Built-ins have no supervisor, so their caller context still enforces the method budget.
func (m *Manager) callContext(parent context.Context, id string, budget supervisor.Budget) (context.Context, context.CancelFunc) {
	if m.IsBuiltIn(id) {
		return context.WithTimeout(parent, budget.Hard+budgetSlack)
	}
	return context.WithCancel(parent)
}
