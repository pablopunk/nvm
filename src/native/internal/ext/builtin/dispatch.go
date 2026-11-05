package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// dispatch maps a protocol method to the extension's interface for it and returns the JSON of the result.
func (r *Runner) dispatch(ctx context.Context, method, query string, invocation *api.Invocation) (json.RawMessage, error) {
	switch method {
	case protocol.MethodCommands:
		return r.commands(ctx)
	case protocol.MethodRoot:
		return r.roots(ctx)
	case protocol.MethodSearch:
		return r.search(ctx, query)
	case protocol.MethodRun:
		return r.run(ctx, invocation)
	case protocol.MethodLoad:
		return r.load(ctx, invocation)
	case protocol.MethodMenuBars:
		return r.menuBars(ctx)
	case protocol.MethodMenuBarTriggers:
		return r.menuBarTriggers()
	case protocol.MethodRestoreWindow:
		return r.restoreWindow(ctx, invocation)
	}
	return nil, fmt.Errorf("unknown method %q: %w", method, api.ErrInvalidInput)
}

func (r *Runner) commands(ctx context.Context) (json.RawMessage, error) {
	provider, ok := r.ext.(api.CommandProvider)
	if !ok {
		return nil, notImplemented(protocol.MethodCommands)
	}
	out, err := provider.Commands(r.context.For(ctx, api.LaunchContext{}))
	return encode(protocol.MethodCommands, out, err)
}

func (r *Runner) roots(ctx context.Context) (json.RawMessage, error) {
	provider, ok := r.ext.(api.RootProvider)
	if !ok {
		return nil, notImplemented(protocol.MethodRoot)
	}
	out, err := provider.RootItems(r.context.For(ctx, api.LaunchContext{}))
	return encode(protocol.MethodRoot, out, err)
}

func (r *Runner) search(ctx context.Context, query string) (json.RawMessage, error) {
	provider, ok := r.ext.(api.SearchProvider)
	if !ok {
		return nil, notImplemented(protocol.MethodSearch)
	}
	out, err := provider.SearchItems(r.context.For(ctx, api.LaunchContext{}), query)
	return encode(protocol.MethodSearch, out, err)
}

func (r *Runner) run(ctx context.Context, invocation *api.Invocation) (json.RawMessage, error) {
	handler, ok := r.ext.(api.ActionHandler)
	if !ok {
		return nil, notImplemented(protocol.MethodRun)
	}
	in := api.Invocation{}
	if invocation != nil {
		in = *invocation
	}
	out, err := handler.RunAction(r.context.For(ctx, in.Launch), in)
	return encode(protocol.MethodRun, out, err)
}

func (r *Runner) load(ctx context.Context, invocation *api.Invocation) (json.RawMessage, error) {
	loader, ok := r.ext.(api.DataLoader)
	if !ok {
		return nil, notImplemented(protocol.MethodLoad)
	}
	id := ""
	if invocation != nil {
		id = invocation.Ref.ID
	}
	out, err := loader.LoadItems(r.context.For(ctx, api.LaunchContext{}), id)
	return encode(protocol.MethodLoad, out, err)
}

func (r *Runner) menuBars(ctx context.Context) (json.RawMessage, error) {
	provider, ok := r.ext.(api.MenuBarProvider)
	if !ok {
		return nil, notImplemented(protocol.MethodMenuBars)
	}
	out, err := provider.MenuBars(r.context.For(ctx, api.LaunchContext{}))
	return encode(protocol.MethodMenuBars, out, err)
}

func (r *Runner) menuBarTriggers() (json.RawMessage, error) {
	provider, ok := r.ext.(api.MenuBarProvider)
	if !ok {
		return nil, notImplemented(protocol.MethodMenuBarTriggers)
	}
	return encode(protocol.MethodMenuBarTriggers, provider.MenuBarTriggers(), nil)
}

func (r *Runner) restoreWindow(ctx context.Context, invocation *api.Invocation) (json.RawMessage, error) {
	restorer, ok := r.ext.(api.WindowRestorer)
	if !ok {
		return nil, notImplemented(protocol.MethodRestoreWindow)
	}
	key := ""
	if invocation != nil {
		key = invocation.Ref.ID
	}
	out, err := restorer.RestoreWindow(r.context.For(ctx, api.LaunchContext{}), key)
	return encode(protocol.MethodRestoreWindow, out, err)
}

// encode marshals a result, or returns the entry point's error. A nil result encodes as null, as a child's does.
func encode(method string, out any, err error) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	data, marshalErr := json.Marshal(out)
	if marshalErr != nil {
		return nil, fmt.Errorf("%s returned data that cannot be encoded: %w", method, api.ErrInvalidInput)
	}
	return data, nil
}

func notImplemented(method string) error {
	return fmt.Errorf("the extension does not implement %s: %w", method, api.ErrNotAvailable)
}
