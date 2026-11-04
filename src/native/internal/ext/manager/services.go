package manager

import (
	"context"

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
