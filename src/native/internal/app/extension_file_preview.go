package app

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/manager"
)

func (p *App) extensionFilePreview(m *manager.Manager) func(string, string, any, func(error)) {
	return func(id, path string, identity any, done func(error)) {
		p.extCalls.Add(1)
		go func() {
			defer p.extCalls.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := m.PreviewFileAction(ctx, id, path, identity)
			p.onUI(func() { done(err) })
		}()
	}
}
