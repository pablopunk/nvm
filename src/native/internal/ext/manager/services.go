package manager

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// Services answers one host-service call of extension id exactly as its child's calls are answered. Host-owned state,
// such as an editor's drafts, is read and written through here by the palette.
func (m *Manager) Services(ctx context.Context, id string, call protocol.ServiceCall) (protocol.ServiceReply, error) {
	m.mu.Lock()
	e, err := m.findLocked(id)
	m.mu.Unlock()
	if err != nil {
		return protocol.ServiceReply{}, err
	}
	if e.host == nil {
		return protocol.ServiceReply{Failure: &api.Failure{Code: api.CodeUnavailable, Message: "this extension has no host services"}}, nil
	}
	return e.host.Serve(ctx, call), nil
}

// callService makes one host-service call of extension extID, the same call its child would make, and decodes the
// result into into when it is not nil. The call is bounded by budget.
func (m *Manager) callService(extID, service, op string, args any, into any, budget time.Duration) error {
	raw, err := json.Marshal(args)
	if err != nil {
		return api.ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	reply, err := m.Services(ctx, extID, protocol.ServiceCall{Service: service, Op: op, Args: raw})
	if err != nil {
		return err
	}
	if reply.Failure != nil {
		return errors.New(reply.Failure.Message)
	}
	if into == nil || len(reply.Data) == 0 {
		return nil
	}
	return json.Unmarshal(reply.Data, into)
}
