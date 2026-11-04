package extitems

import (
	"context"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/menubar"
)

// runBudget bounds one menu row action that an extension runs. Its result is logged, never shown: a menu cannot host a
// view, so an action that opens one is reported instead.
const runBudget = 5 * time.Second

// invoke maps a declared action of extension id to the host's click handler. Run actions go to the extension; open
// URL and copy text are host operations; every other kind has no meaning in a menu, so its row stays disabled.
func (s *Service) invoke(id string, action api.Action) func() {
	switch action.Kind {
	case api.ActionRun:
		if action.Ref == nil {
			return nil
		}
		ref := *action.Ref
		return func() { go s.runAction(id, ref) }
	case api.ActionOpenURL:
		url := action.URL
		return menubar.UIAction(func() { s.opts.Hooks.openURL(url) }, s.opts.OnUI)
	case api.ActionCopyText:
		text := action.Text
		return menubar.UIAction(func() { s.opts.Hooks.copyText(text) }, s.opts.OnUI)
	}
	return nil
}

// runAction calls the extension's RunAction for a menu row, off the UI thread, and logs what did not succeed.
func (s *Service) runAction(id string, ref api.ActionRef) {
	ctx, cancel := context.WithTimeout(context.Background(), runBudget)
	defer cancel()
	result, failure, err := s.opts.Source.Run(ctx, id, api.Invocation{Ref: ref})
	switch {
	case err != nil:
		s.logs().Warn("menu bar action failed", fieldsOf(id, err.Error()))
	case failure != nil:
		s.logs().Warn("menu bar action failed", fieldsOf(id, failure.Message))
	case result.View != nil:
		s.logs().Info("menu bar actions cannot open a view", fieldsOf(id, ref.ID))
	}
}
