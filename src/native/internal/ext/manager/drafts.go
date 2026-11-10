package manager

import (
	"time"

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

// callDrafts makes one drafts call on the extension's services, the same call its child would make.
func (m *Manager) callDrafts(extID, op string, args any, into any) error {
	return m.callService(extID, protocol.ServiceDrafts, op, args, into, draftCallBudget)
}
