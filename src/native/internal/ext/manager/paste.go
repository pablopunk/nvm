package manager

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
	"github.com/pablopunk/nvm/native/internal/ext/services"
)

// pasteCallBudget bounds one paste or typing action. Typing with a delay between characters runs for a while.
const pasteCallBudget = 2 * time.Minute

// PerformPaste has the services of extension extID send a paste or typing action to the app in front. The palette
// calls it after it hides, so the keys reach the app the content came from. The error is the HUD message.
func (m *Manager) PerformPaste(extID string, action api.Action) error {
	raw, err := json.Marshal(action)
	if err != nil {
		return api.ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(context.Background(), pasteCallBudget)
	defer cancel()
	reply, err := m.Services(ctx, extID, protocol.ServiceCall{Service: protocol.ServiceDesktopPaste, Op: protocol.OpPerform, Args: raw})
	if err != nil {
		return err
	}
	if reply.Failure != nil {
		return errors.New(reply.Failure.Message)
	}
	return nil
}

// PerformPasteForIdentity admits the service of the installation that created the action.
func (m *Manager) PerformPasteForIdentity(extID string, identity any, action api.Action) error {
	raw, err := json.Marshal(action)
	if err != nil {
		return api.ErrInvalidInput
	}
	m.mu.Lock()
	e, err := m.findLocked(extID)
	var actual any
	if err == nil && e.snapshotIdentity != nil {
		actual = e.snapshotIdentity
	}
	if err != nil || !m.isRunnable(e) || e.host == nil || !sameInvocationIdentity(actual, identity) {
		m.mu.Unlock()
		return ErrChangedInstallation
	}
	host := e.host
	if e.revoked == nil {
		e.revoked = &atomic.Bool{}
	}
	revoked := e.revoked
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), pasteCallBudget)
	defer cancel()
	ctx = services.WithPasteAdmission(ctx, func() bool { return !revoked.Load() })
	reply := host.Serve(ctx, protocol.ServiceCall{Service: protocol.ServiceDesktopPaste, Op: protocol.OpPerform, Args: raw})
	if reply.Failure != nil {
		return errors.New(reply.Failure.Message)
	}
	return nil
}
