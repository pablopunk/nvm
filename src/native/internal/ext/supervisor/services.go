package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// ServiceHandler answers the host services a child calls while a request runs. It runs on the request's goroutine,
// and the request's run budget does not count the time it takes; the child waits for the reply. The runtime fills in
// ReplyTo, so a handler sets only Data or Failure.
type ServiceHandler interface {
	Serve(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply
}

// serviceCallFrom reports whether a child line is a service call that the running request must answer.
func serviceCallFrom(line lineResult, open bool) (protocol.ServiceCall, bool) {
	if !open || line.err != nil {
		return protocol.ServiceCall{}, false
	}
	return protocol.ParseServiceCall(line.data)
}

// serviceCallTimeout bounds one host service call. It is the service's own limit, not the caller's run budget: a shell
// command or an OCR read may take longer than a run is allowed to compute, and the run budget does not count that wait.
const serviceCallTimeout = 3 * time.Minute

// serveCall dispatches to the handler, or refuses the call when the runtime has none.
func (r *Runtime) serveCall(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	if r.services == nil {
		return protocol.ServiceReply{Failure: &api.Failure{Code: api.CodeUnavailable, Message: "host services are not available to this extension"}}
	}
	return r.services.Serve(ctx, call)
}

// slowServiceCall is how long one host service call may hold a request before it is worth a log line.
const slowServiceCall = 2 * time.Second

// noteSlowServiceCall names a service call that held a request up, because a request holds the extension's only slot
// and everything behind it, an opening view included, waits for it. A file call names the file it was reading.
func (r *Runtime) noteSlowServiceCall(call protocol.ServiceCall, took time.Duration) {
	if took < slowServiceCall {
		return
	}
	fields := logx.F{"service": call.Service, "op": call.Op, "duration_ms": took.Milliseconds()}
	var args protocol.PathArgs
	if call.Service == protocol.ServiceDesktopFiles && json.Unmarshal(call.Args, &args) == nil && args.Path != "" {
		fields["file"] = filepath.Base(args.Path)
	}
	r.logLifecycle(logx.Warn, "extension service call slow", fields)
}

// noteSlowSlot records a request that sat behind another one of the same extension.
func (r *Runtime) noteSlowSlot(method string, took time.Duration, outcome string) {
	if took >= slowServiceCall {
		r.logLifecycle(logx.Warn, "extension request queued", logx.F{"method": method, "outcome": outcome, "duration_ms": took.Milliseconds()})
	}
}

// lifetimeContext returns the context that ends when the runtime stops. Host waits on behalf of a call end with it.
func (r *Runtime) lifetimeContext() context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lifetime
}

// answerViewService lets a stopped runtime or canceled view leave even when a filesystem call cannot be interrupted.
// At most four abandoned host calls can remain outstanding for one runtime.
func (r *Runtime) answerViewService(parent, lifetime context.Context, child *childProcess, call protocol.ServiceCall, method string) (protocol.ServiceReply, error) {
	if lifetime.Err() != nil {
		return protocol.ServiceReply{}, lifetime.Err()
	}
	started := r.cfg.Clock.Now()
	defer func() { r.noteSlowServiceCall(call, r.cfg.Clock.Now().Sub(started)) }()
	ctx, cancel := context.WithTimeout(protocol.WithLifetime(protocol.WithRequestMethod(parent, method), lifetime), serviceCallTimeout)
	defer cancel()
	stop := context.AfterFunc(lifetime, cancel)
	defer stop()
	select {
	case r.viewServices <- struct{}{}:
	case <-ctx.Done():
		return protocol.ServiceReply{}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		<-r.viewServices
		return protocol.ServiceReply{}, err
	}
	result := make(chan protocol.ServiceReply, 1)
	go func() { defer func() { <-r.viewServices }(); result <- r.serveCall(ctx, call) }()
	var reply protocol.ServiceReply
	select {
	case reply = <-result:
	case <-ctx.Done():
		return protocol.ServiceReply{}, ctx.Err()
	}
	reply.ReplyTo = call.CallID
	line, err := json.Marshal(reply)
	if err != nil {
		line = []byte(fmt.Sprintf(`{"replyTo":%d,"failure":{"code":"internal","message":"reply cannot be encoded"}}`, call.CallID))
	}
	if err := child.proc.WriteLine(line); err != nil {
		return reply, r.failChild(child, api.CodeInternal, "cannot answer a service call: "+err.Error())
	}
	return reply, nil
}
