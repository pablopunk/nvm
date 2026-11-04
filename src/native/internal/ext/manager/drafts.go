package manager

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// draftCallBudget bounds one host-side draft write or read. The store is a local file, so the bound is generous.
const draftCallBudget = 5 * time.Second

// LoadDraft returns the draft the extension's editor left under key, if any.
func (m *Manager) LoadDraft(extID, key string) (protocol.DraftLookup, error) {
	var lookup protocol.DraftLookup
	err := m.callDrafts(extID, protocol.OpGet, protocol.KeyArgs{Key: key}, &lookup)
	return lookup, err
}

// SaveDraft stores the unsaved text of an editor, flushed before it returns.
func (m *Manager) SaveDraft(extID string, args protocol.DraftSaveArgs) error {
	return m.callDrafts(extID, protocol.OpSave, args, nil)
}

// DiscardDraft drops the draft under key.
func (m *Manager) DiscardDraft(extID, key string) error {
	return m.callDrafts(extID, protocol.OpDiscard, protocol.KeyArgs{Key: key}, nil)
}

// callDrafts makes one drafts call on the extension's services, the same call its child would make, and decodes the
// result into into when it is not nil.
func (m *Manager) callDrafts(extID, op string, args any, into any) error {
	raw, err := json.Marshal(args)
	if err != nil {
		return api.ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(context.Background(), draftCallBudget)
	defer cancel()
	reply, err := m.Services(ctx, extID, protocol.ServiceCall{Service: protocol.ServiceDrafts, Op: op, Args: raw})
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
