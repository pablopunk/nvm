package manager

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
)

// ToggleSettingAction operates on the enabled installation that produced the action, never a replacement with the same ID.
func (m *Manager) ToggleSettingAction(id, key string, identity any) error {
	m.mu.Lock()
	e, err := m.findLocked(id)
	var revision *byte
	if identity != nil {
		revision, _ = identity.(*byte)
	}
	if err != nil || !m.isRunnable(e) || e.host == nil || (!e.builtin && (revision == nil || revision != e.snapshotIdentity)) {
		m.mu.Unlock()
		return errors.New("this extension action has expired; reopen its view")
	}
	host := e.host
	if e.revoked == nil {
		e.revoked = &atomic.Bool{}
	}
	revoked := e.revoked
	m.mu.Unlock()
	raw, err := json.Marshal(protocol.KeyArgs{Key: key})
	if err != nil {
		return api.ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingCallBudget)
	defer cancel()
	ctx = services.WithActionAdmission(ctx, func() bool { return !revoked.Load() })
	reply := host.Serve(ctx, protocol.ServiceCall{Service: protocol.ServiceSettings, Op: protocol.OpToggle, Args: raw})
	if reply.Failure != nil {
		return errors.New(reply.Failure.Message)
	}
	return nil
}
