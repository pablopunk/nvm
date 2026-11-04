package manager

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

// SearchResult is one enabled extension's answer to a query. Failure is set when the extension timed out,
// crashed, was not running, or returned an error; Items is then empty. A failure never hides other results.
type SearchResult struct {
	ExtensionID string
	Items       []api.Item
	Failure     *api.Failure
}

// Search asks every enabled, running extension for its items. The calls run in parallel, each bounded by its
// own Search budget (B8: 100 ms hard), so one hanging extension cannot delay the others past that budget. It
// returns when every call has answered or timed out, sorted by extension id.
func (m *Manager) Search(ctx context.Context, query string) []SearchResult {
	targets, budget := m.searchTargets()
	results := make(chan SearchResult, len(targets))
	var wg sync.WaitGroup
	for _, target := range targets {
		wg.Add(1)
		go func(target runtimeTarget) {
			defer wg.Done()
			results <- m.searchOne(ctx, target, query, budget)
		}(target)
	}
	wg.Wait()
	close(results)
	collected := make([]SearchResult, 0, len(targets))
	for result := range results {
		collected = append(collected, result)
	}
	sort.Slice(collected, func(i, j int) bool { return collected[i].ExtensionID < collected[j].ExtensionID })
	return collected
}

// runtimeTarget is one enabled runtime and the extension it belongs to.
type runtimeTarget struct {
	id      string
	runtime *supervisor.Runtime
}

// searchTargets snapshots the enabled runtimes and the Search budget. An extension that is still starting or
// was never started is left out: a normal startup is not an error to show in the palette.
func (m *Manager) searchTargets() ([]runtimeTarget, supervisor.Budget) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var targets []runtimeTarget
	for _, key := range m.sortedKeysLocked() {
		e := m.entries[key]
		if e.runtime != nil && m.isEnabled(e.bundle.Manifest.ID) && isAnswerable(e.runtime.Status().State) {
			targets = append(targets, runtimeTarget{id: e.bundle.Manifest.ID, runtime: e.runtime})
		}
	}
	return targets, m.config.BudgetFor(protocol.MethodSearch)
}

func isAnswerable(state supervisor.State) bool {
	return state != supervisor.StateIdle && state != supervisor.StateStarting
}

func (m *Manager) searchOne(ctx context.Context, target runtimeTarget, query string, budget supervisor.Budget) SearchResult {
	result := SearchResult{ExtensionID: target.id}
	callCtx, cancel := context.WithTimeout(ctx, budget.Hard+budgetSlack)
	defer cancel()
	response, err := target.runtime.Call(callCtx, protocol.MethodSearch, query, nil)
	if err != nil {
		result.Failure = failureOf(err)
		return result
	}
	if response.Failure != nil {
		result.Failure = response.Failure
		return result
	}
	if err := json.Unmarshal(response.Data, &result.Items); err != nil {
		result.Failure = &api.Failure{Code: api.CodeInternal, Message: "search returned items that are not valid"}
	}
	return result
}

// budgetSlack lets a call's context outlive the hard budget by a few milliseconds, so the child's own timeout
// reports first and the caller sees a timeout failure rather than a bare context error.
const budgetSlack = 10 * time.Millisecond

// Run executes one action on an enabled extension and returns its result. The invocation carries the action
// reference and its arguments as data; nothing executable crosses the boundary.
func (m *Manager) Run(ctx context.Context, id string, invocation api.Invocation) (api.Result, *api.Failure, error) {
	runtime, err := m.runtimeFor(id)
	if err != nil {
		return api.Result{}, nil, err
	}
	budget := m.config.BudgetFor(protocol.MethodRun)
	callCtx, cancel := context.WithTimeout(ctx, budget.Hard+budgetSlack)
	defer cancel()
	response, err := runtime.Call(callCtx, protocol.MethodRun, "", &invocation)
	if err != nil {
		return api.Result{}, failureOf(err), nil
	}
	if response.Failure != nil {
		return api.Result{}, response.Failure, nil
	}
	var result api.Result
	if err := json.Unmarshal(response.Data, &result); err != nil {
		return api.Result{}, &api.Failure{Code: api.CodeInternal, Message: "run returned a result that is not valid"}, nil
	}
	return result, nil, nil
}

func (m *Manager) runtimeFor(id string) (*supervisor.Runtime, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil {
		return nil, err
	}
	if e.runtime == nil {
		return nil, ErrUnknownExtension
	}
	return e.runtime, nil
}

// WaitFor blocks until the runtime of extension id reports a status that satisfies pred, or until ctx ends.
// It exists for the manager view and for tests; the palette never waits on an extension.
func (m *Manager) WaitFor(ctx context.Context, id string, pred func(supervisor.Status) bool) error {
	runtime, err := m.runtimeFor(id)
	if err != nil {
		return err
	}
	return runtime.WaitFor(ctx, pred)
}

// failureOf converts a host-side call error into the wire failure the palette shows. A context that ended is a
// timeout or a cancellation, never an internal error.
func failureOf(err error) *api.Failure {
	var callErr *supervisor.CallError
	switch {
	case errors.As(err, &callErr):
		failure := callErr.Failure
		return &failure
	case errors.Is(err, context.DeadlineExceeded):
		return &api.Failure{Code: api.CodeTimeout, Message: "the extension did not answer in time"}
	case errors.Is(err, context.Canceled):
		return &api.Failure{Code: api.CodeCancelled, Message: "the request was cancelled"}
	}
	failure := api.FailureOf(err)
	return &failure
}
