package child

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// maxFailureMessage bounds the text a failure carries to the host. Interpreter errors can be long.
const maxFailureMessage = 2 << 10

// answer runs one request and returns its response. A panic inside extension code is recovered here and
// reported as an internal failure with a reference; the child stays alive for the next request.
func (e *extension) answer(req protocol.Request) (response protocol.Response) {
	response.ID = req.ID
	defer func() {
		if recovered := recover(); recovered != nil {
			response = protocol.Response{ID: req.ID, Failure: panicFailure(recovered)}
		}
	}()
	data, err := e.dispatch(req)
	if err != nil {
		failure := api.FailureOf(err)
		failure.Message = truncate(failure.Message)
		response.Failure = &failure
		return response
	}
	response.Data = data
	return response
}

// dispatch maps a method to its entry point and returns the JSON of the result.
func (e *extension) dispatch(req protocol.Request) (json.RawMessage, error) {
	switch req.Method {
	case protocol.MethodCommands:
		return e.encode(protocol.MethodCommands, e.contextArgument(api.LaunchContext{}))
	case protocol.MethodRoot:
		return e.encode(protocol.MethodRoot, e.contextArgument(api.LaunchContext{}))
	case protocol.MethodSearch:
		return e.encode(protocol.MethodSearch, e.contextArgument(api.LaunchContext{}), reflect.ValueOf(req.Query))
	case protocol.MethodRun:
		return e.runAction(req.Invocation)
	case protocol.MethodLoad:
		return e.loadItems(req.Invocation)
	case protocol.MethodMenuBars:
		return e.encode(protocol.MethodMenuBars, e.contextArgument(api.LaunchContext{}))
	case protocol.MethodMenuBarTriggers:
		return e.encode(protocol.MethodMenuBarTriggers)
	}
	return nil, fmt.Errorf("unknown method %q: %w", req.Method, api.ErrInvalidInput)
}

// runAction passes the invocation to RunAction as the api.Invocation value it declares.
func (e *extension) runAction(invocation *api.Invocation) (json.RawMessage, error) {
	in := api.Invocation{}
	if invocation != nil {
		in = *invocation
	}
	return e.encode(protocol.MethodRun, e.contextArgument(in.Launch), reflect.ValueOf(in))
}

// loadItems passes the loader named by the invocation to LoadItems. The loader id travels in Ref.ID.
func (e *extension) loadItems(invocation *api.Invocation) (json.RawMessage, error) {
	loader := ""
	if invocation != nil {
		loader = invocation.Ref.ID
	}
	return e.encode(protocol.MethodLoad, e.contextArgument(api.LaunchContext{}), reflect.ValueOf(loader))
}

// encode calls an entry point and marshals its first result. Results are JSON before they leave the child, so
// the host validates wire data and never sees interpreter values.
func (e *extension) encode(method string, args ...reflect.Value) (json.RawMessage, error) {
	out, err := e.invoke(method, args...)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return json.RawMessage("null"), nil
	}
	data, err := json.Marshal(out[0].Interface())
	if err != nil {
		return nil, fmt.Errorf("%s returned data that cannot be encoded: %w", method, err)
	}
	return data, nil
}

// panicFailure reports a recovered panic. The message names the panic and never the arguments; the reference
// is random so a support request can name one incident.
func panicFailure(recovered any) *api.Failure {
	return &api.Failure{
		Code:      api.CodeInternal,
		Message:   truncate(fmt.Sprintf("extension panicked: %v", recovered)),
		Reference: newReference(),
	}
}

func newReference() string {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "ext-panic-unknown"
	}
	return "ext-panic-" + hex.EncodeToString(raw[:])
}

func truncate(text string) string {
	if len(text) <= maxFailureMessage {
		return text
	}
	return text[:maxFailureMessage] + "..."
}
