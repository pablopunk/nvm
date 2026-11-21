package app

import (
	"context"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"time"
)

func (p *App) extensionShellAction(m *manager.Manager) func(context.Context, string, api.ShellCommand, any, func(api.ShellResult, error)) {
	return func(base context.Context, id string, command api.ShellCommand, identity any, done func(api.ShellResult, error)) {
		p.extCalls.Add(1)
		go func() {
			defer p.extCalls.Done()
			ctx, cancel := context.WithTimeout(base, 121*time.Second)
			defer cancel()
			result, err := m.ShellAction(ctx, id, command, identity)
			p.onUI(func() { done(result, err) })
		}()
	}
}
