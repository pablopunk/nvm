package supervisor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// ServiceHandler answers the host services a child calls while a request runs. It runs on the request's goroutine
// inside the request's budget, and it must return promptly: the child waits for the reply. The runtime fills in
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

// answerService runs one service call and writes its reply to the child. The services get the request's hard
// budget as their deadline, so a slow service ends with the call instead of outliving it.
func (r *Runtime) answerService(child *childProcess, call protocol.ServiceCall, budget Budget, method string) error {
	ctx, cancel := context.WithTimeout(protocol.WithRequestMethod(context.Background(), method), budget.Hard)
	defer cancel()
	reply := r.serveCall(ctx, call)
	reply.ReplyTo = call.CallID
	line, err := json.Marshal(reply)
	if err != nil {
		line = []byte(fmt.Sprintf(`{"replyTo":%d,"failure":{"code":"internal","message":"reply cannot be encoded"}}`, call.CallID))
	}
	if err := child.proc.WriteLine(line); err != nil {
		return r.failChild(child, api.CodeInternal, "cannot answer a service call: "+err.Error())
	}
	return nil
}

// serveCall dispatches to the handler, or refuses the call when the runtime has none.
func (r *Runtime) serveCall(ctx context.Context, call protocol.ServiceCall) protocol.ServiceReply {
	if r.services == nil {
		return protocol.ServiceReply{Failure: &api.Failure{Code: api.CodeUnavailable, Message: "host services are not available to this extension"}}
	}
	return r.services.Serve(ctx, call)
}
