package app

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	systemfeat "github.com/pablopunk/nvm/native/internal/features/system"
	"github.com/pablopunk/nvm/native/internal/platform"
	"time"
)

func (p *App) extensionSystemCommand(m *manager.Manager) func(string, string, string, any, func(string)) {
	return func(id, op, pane string, identity any, done func(string)) {
		p.extCalls.Add(1)
		go func() {
			defer p.extCalls.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := m.SystemCommandAction(ctx, id, op, pane, identity)
			failure := ""
			if err != nil {
				command, _ := systemfeat.CommandForOp(op)
				title := systemfeat.Title(command, platform.ShellLabelsFor(p.goos).SettingsCommand)
				failure = describeSystemFailure(title, err)
			}
			p.onUI(func() { done(failure) })
		}()
	}
}
