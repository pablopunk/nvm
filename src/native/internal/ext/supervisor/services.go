package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
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

// answerService runs one service call and writes its reply to the child. A slow service ends at serviceCallTimeout
// or when the runtime stops, so it never outlives the host.
func (r *Runtime) answerService(child *childProcess, call protocol.ServiceCall, budget Budget, method string) (reply protocol.ServiceReply, err error) {
	ctx, cancel := context.WithTimeout(protocol.WithLifetime(protocol.WithRequestMethod(context.Background(), method), r.lifetimeContext()), serviceCallTimeout)
	defer cancel()
	reply = r.serveCall(ctx, call)
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

// serveCall dispatches to the handler, or refuses the call when the runtime has none.
func (r *Runtime) serveCall(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	if r.services == nil {
		return protocol.ServiceReply{Failure: &api.Failure{Code: api.CodeUnavailable, Message: "host services are not available to this extension"}}
	}
	return r.services.Serve(ctx, call)
}

// lifetimeContext returns the context that ends when the runtime stops. Host waits on behalf of a call end with it.
func (r *Runtime) lifetimeContext() context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lifetime
}

// answerViewService allows a canceled view to leave even when a filesystem call cannot be interrupted.
// At most four such host calls can remain outstanding for one runtime.
func (r *Runtime) answerViewService(parent context.Context, child *childProcess, call protocol.ServiceCall, method string) (protocol.ServiceReply, error) {
	ctx, cancel := context.WithTimeout(protocol.WithLifetime(protocol.WithRequestMethod(parent, method), r.lifetimeContext()), serviceCallTimeout)
	defer cancel()
	select {
	case r.viewServices <- struct{}{}:
	case <-ctx.Done():
		return protocol.ServiceReply{}, ctx.Err()
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
