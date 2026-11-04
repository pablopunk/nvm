package app

import (
	"context"
	"time"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/palette/extview"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

// extensionRunTimeout bounds one action an extension runs from the palette. Actions run off the UI thread.
const extensionRunTimeout = 10 * time.Second

// extensionViewDeps are the services of extension views: an action runs on the manager's supervised child, and its
// result returns to the UI thread, where the palette applies it.
func (p *App) extensionViewDeps(m *manager.Manager) extview.Deps {
	return extview.Deps{
		Invoke: func(extID string, in api.Invocation, done func(api.Result, *api.Failure)) {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), extensionRunTimeout)
				defer cancel()
				result, failure, err := m.Run(ctx, extID, in)
				if err != nil {
					failure = &api.Failure{Code: api.CodeInternal, Message: err.Error()}
				}
				p.onUI(func() { done(result, failure) })
			}()
		},
		OpenURL:  func(url string) { _ = mygo.Shell.OpenExternal(url) },
		CopyText: func(text string) { mygo.Clipboard.WriteText(text) },
		Files:    real.NewPorts().FS,
	}
}
