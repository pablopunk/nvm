// Package builtin runs the extensions compiled into Nevermind. A built-in is trusted and bundled: it runs in the host
// process, is always enabled, needs no review, and answers the same protocol methods as an extension child. Its
// service calls go through the same handler as a child's, so a built-in has no door the API does not have.
//
// Layer: ext. The package imports api, child, protocol, supervisor, platform and the standard library.
package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/child"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// Runner is the runtime of one built-in extension. It satisfies the calls the manager makes of a child runtime.
// There is no process to start or stop, so lifecycle calls do nothing and its state is always running.
type Runner struct {
	ext     api.Extension
	context child.DirectContext
}

// New prepares the runtime of ext. serve answers the extension's service calls in the host process.
func New(ext api.Extension, clock platform.Clock, serve func(protocol.ServiceCall) protocol.ServiceReply) *Runner {
	return &Runner{
		ext:     ext,
		context: child.NewDirectContext(ext.Manifest().Identity, clock, serve),
	}
}

// Provides reports whether the extension implements the protocol method.
func (r *Runner) Provides(method string) bool {
	switch method {
	case protocol.MethodCommands:
		_, ok := r.ext.(api.CommandProvider)
		return ok
	case protocol.MethodRoot:
		_, ok := r.ext.(api.RootProvider)
		return ok
	case protocol.MethodSearch:
		_, ok := r.ext.(api.SearchProvider)
		return ok
	case protocol.MethodRun:
		_, ok := r.ext.(api.ActionHandler)
		return ok
	case protocol.MethodLoad:
		_, ok := r.ext.(api.DataLoader)
		return ok
	case protocol.MethodMenuBars, protocol.MethodMenuBarTriggers:
		_, ok := r.ext.(api.MenuBarProvider)
		return ok
	case protocol.MethodRowActions:
		_, ok := r.ext.(api.RowActionProvider)
		return ok
	}
	return false
}

// Status is always running: a built-in has no start-up to fail.
func (r *Runner) Status() supervisor.Status {
	return supervisor.Status{State: supervisor.StateRunning}
}

// Start, Stop and Reset do nothing: a built-in is compiled in and always available.
func (r *Runner) Start() {}

// Stop does nothing; see Start.
func (r *Runner) Stop() {}

// Reset does nothing; see Start.
func (r *Runner) Reset() {}

// WaitFor returns once pred holds for the status, which for a built-in is at once, or when ctx ends.
func (r *Runner) WaitFor(ctx context.Context, pred func(supervisor.Status) bool) error {
	if pred(r.Status()) {
		return nil
	}
	<-ctx.Done()
	return ctx.Err()
}

// Call runs one protocol method on the calling goroutine's behalf. The work runs on its own goroutine so that ctx,
// which carries the budget, ends the call even when the extension is slow. A built-in cannot be killed, so an overrun
// call finishes in the background and its answer is dropped.
func (r *Runner) Call(ctx context.Context, method, query string, invocation *api.Invocation) (protocol.Response, error) {
	answered := make(chan protocol.Response, 1)
	go func() { answered <- r.answer(ctx, method, query, invocation) }()
	select {
	case response := <-answered:
		return response, nil
	case <-ctx.Done():
		return protocol.Response{}, ctx.Err()
	}
}

// Load runs a data loader and returns its items, as a child's Load does.
func (r *Runner) Load(ctx context.Context, loader string) ([]api.Item, error) {
	response, err := r.Call(ctx, protocol.MethodLoad, "", &api.Invocation{Ref: api.ActionRef{ID: loader}})
	if err != nil {
		return nil, err
	}
	if response.Failure != nil {
		return nil, &supervisor.CallError{Failure: *response.Failure}
	}
	var items []api.Item
	if err := json.Unmarshal(response.Data, &items); err != nil {
		return nil, fmt.Errorf("load returned items that are not valid: %w", api.ErrInvalidInput)
	}
	return items, nil
}

// answer runs one method and encodes its result. A panic in the extension becomes an internal failure.
func (r *Runner) answer(ctx context.Context, method, query string, invocation *api.Invocation) (response protocol.Response) {
	defer func() {
		if recovered := recover(); recovered != nil {
			response = protocol.Response{Failure: child.PanicFailure(recovered)}
		}
	}()
	data, err := r.dispatch(ctx, method, query, invocation)
	if err != nil {
		failure := api.FailureOf(err)
		return protocol.Response{Failure: &failure}
	}
	return protocol.Response{Data: data}
}
