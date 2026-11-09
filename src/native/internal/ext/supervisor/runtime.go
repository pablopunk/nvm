package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// Runtime owns the child process of one extension. Calls are serialized: a child answers one request at a
// time. Every method is safe for concurrent use; Call never blocks past its context.
type Runtime struct {
	cfg          Config
	sourcePath   string
	identity     api.Identity
	services     ServiceHandler
	slots        chan struct{}
	viewServices chan struct{}
	mu           sync.Mutex
	state        State
	reason       string
	generation   uint64
	child        *childProcess
	declared     []string
	failures     []time.Time
	retryAt      time.Time
	diagnostics  []string
	nextRequest  uint64
	changed      chan struct{}
	// lifetime ends when the extension stops; host waits on behalf of a call end with it.
	lifetime       context.Context
	cancelLifetime context.CancelFunc
}

// New prepares a runtime for the extension source at sourcePath. Nothing runs until Start. services answers the
// extension's service calls; nil refuses them.
func New(cfg Config, identity api.Identity, sourcePath string, services ServiceHandler) *Runtime {
	r := &Runtime{
		cfg:          cfg,
		sourcePath:   sourcePath,
		identity:     identity,
		services:     services,
		slots:        make(chan struct{}, 1),
		viewServices: make(chan struct{}, 4),
		state:        StateIdle,
		changed:      make(chan struct{}),
	}
	r.lifetime, r.cancelLifetime = context.WithCancel(context.Background())
	return r
}

// StartDormant marks the extension ready to start on its first call. declared lists the methods its source implements,
// which is what Provides answers until a child reports its own.
func (r *Runtime) StartDormant(declared []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state != StateIdle {
		return
	}
	r.declared = declared
	r.state = StateDormant
	r.notifyLocked()
}

// Start launches the child in the background and returns at once. It does nothing while the extension is
// running, starting, broken or disabled, and while a failed child's backoff has not passed.
func (r *Runtime) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.startLocked()
}

// Stop kills the child and returns the extension to idle. Calls already waiting see a stopped extension.
func (r *Runtime) Stop() {
	r.mu.Lock()
	r.cancelLifetime()
	r.lifetime, r.cancelLifetime = context.WithCancel(context.Background())
	r.generation++
	child := r.detachChildLocked()
	r.state = StateIdle
	r.reason = ""
	r.notifyLocked()
	r.mu.Unlock()
	if child != nil {
		_ = child.proc.Kill()
	}
}

// Reset clears recorded failures and a disabled or broken state. The extension stays idle; Start runs it again.
func (r *Runtime) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == StateDisabled || r.state == StateBroken || r.state == StateFailed {
		r.state = StateIdle
	}
	r.failures = nil
	r.reason = ""
	r.retryAt = time.Time{}
	r.notifyLocked()
}

// Provides reports whether the running child declared the protocol method. A runtime that is not running provides
// nothing: its declarations are read again when it starts.
func (r *Runtime) Provides(method string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.child == nil {
		return r.state == StateDormant && slices.Contains(r.declared, method)
	}
	return slices.Contains(r.child.methods, method)
}

// Status returns the current state, the reason for a failed, broken or disabled state, and recent diagnostics.
func (r *Runtime) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.statusLocked()
}

func (r *Runtime) statusLocked() Status {
	return Status{
		State:       r.state,
		Reason:      r.reason,
		Failures:    len(r.failures),
		Diagnostics: append([]string(nil), r.diagnostics...),
	}
}

// WaitFor blocks until pred holds for the status, or until ctx ends. It wakes on every state change, so callers
// never poll.
func (r *Runtime) WaitFor(ctx context.Context, pred func(Status) bool) error {
	for {
		r.mu.Lock()
		status := r.statusLocked()
		changed := r.changed
		r.mu.Unlock()
		if pred(status) {
			return nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// notifyLocked wakes every WaitFor caller and the state hook. The caller holds r.mu and has changed the state.
func (r *Runtime) notifyLocked() {
	close(r.changed)
	r.changed = make(chan struct{})
	if r.cfg.OnStateChange != nil {
		go r.cfg.OnStateChange()
	}
}

// Call sends one request and returns the child's response. A child-side failure comes back inside the
// response; a host-side failure (not running, timed out, crashed, cancelled) comes back as an error that is
// a *CallError. The ctx bounds the wait only: a call that outlives its context still finishes inside its
// budget, so the next call never overlaps it.
func (r *Runtime) Call(ctx context.Context, method, query string, invocation *api.Invocation) (protocol.Response, error) {
	return r.Do(ctx, protocol.Request{Method: method, Query: query, Invocation: invocation})
}

// Do sends a prepared request, assigning its id, and returns the child's response. The rules of Call apply.
func (r *Runtime) Do(ctx context.Context, req protocol.Request) (protocol.Response, error) {
	r.mu.Lock()
	lifetime := r.lifetime
	r.mu.Unlock()
	if _, _, err := r.childForCall(ctx, lifetime, req.Method); err != nil {
		return protocol.Response{}, err
	}
	queued := r.cfg.Clock.Now()
	select {
	case r.slots <- struct{}{}:
		r.noteSlowSlot(req.Method, r.cfg.Clock.Now().Sub(queued), "waited")
	case <-ctx.Done():
		r.noteSlowSlot(req.Method, r.cfg.Clock.Now().Sub(queued), "gave up")
		return protocol.Response{}, ctx.Err()
	case <-lifetime.Done():
		return protocol.Response{}, unavailable("stopped")
	}
	if err := ctx.Err(); err != nil {
		<-r.slots
		return protocol.Response{}, err
	}
	req.ID = r.newRequestID()
	result := make(chan callResult, 1)
	gate := &dispatchGate{}
	onDenied := func(failure *api.SnapshotDenial) {
		if req.Invocation != nil && req.Invocation.ProtectSnapshot && req.Invocation.OnSnapshotDenied != nil {
			req.Invocation.OnSnapshotDenied(failure)
		}
	}
	go func() {
		defer func() { <-r.slots }()
		exchangeCtx := context.Background()
		if req.Invocation != nil && req.Invocation.AwaitInitialView {
			exchangeCtx = ctx
		}
		response, err := r.exchange(ctx, exchangeCtx, lifetime, gate, req, onDenied)
		result <- callResult{response: response, err: err}
	}()
	select {
	case outcome := <-result:
		return outcome.response, outcome.err
	case <-ctx.Done():
		gate.cancel()
		return protocol.Response{}, ctx.Err()
	}
}

// Load runs the extension's data loader and returns its items. A child-side failure is a *CallError too, so the
// caller shows one kind of error for every failed call.
func (r *Runtime) Load(ctx context.Context, loader string) ([]api.Item, error) {
	response, err := r.Do(ctx, protocol.Request{Method: protocol.MethodLoad, Invocation: &api.Invocation{Ref: api.ActionRef{ID: loader}}})
	if err != nil {
		return nil, err
	}
	if response.Failure != nil {
		return nil, &CallError{Failure: *response.Failure}
	}
	var items []api.Item
	if err := json.Unmarshal(response.Data, &items); err != nil {
		return nil, &CallError{Failure: api.Failure{Code: api.CodeInternal, Message: "loader returned items that do not decode"}}
	}
	return items, nil
}

type callResult struct {
	response protocol.Response
	err      error
}

func (r *Runtime) newRequestID() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextRequest++
	return r.nextRequest
}

// addDiagnosticLocked keeps the newest diagnostics only.
func (r *Runtime) addDiagnosticLocked(text string) {
	r.diagnostics = append(r.diagnostics, text)
	if extra := len(r.diagnostics) - maxDiagnostics; extra > 0 {
		r.diagnostics = r.diagnostics[extra:]
	}
}

// unavailable is the error for a call that cannot reach a running child.
func unavailable(reason string) error {
	return &CallError{Failure: api.Failure{Code: api.CodeUnavailable, Message: reason}}
}

// CallError is a host-side call failure, such as a timeout, a crash or an extension that is not running.
type CallError struct {
	Failure api.Failure
}

func (e *CallError) Error() string {
	return fmt.Sprintf("%s: %s", e.Failure.Code, e.Failure.Message)
}
