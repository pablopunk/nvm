package manager

import (
	"errors"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"reflect"
)

// SnapshotRevision binds a durable snapshot to the enabled installation that supplied the given in-memory identity.
// A caller must still validate OS access to each file before exposing any persisted content.
func (m *Manager) SnapshotRevision(id string, identity any) (string, bool) {
	selected, ok := identity.(*byte)
	if !ok || selected == nil {
		return "", false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil || e.builtin || e.runtime == nil || !m.isRunnable(e) || selected != e.snapshotIdentity || !e.bundle.Runnable() || e.bundle.Hash == "" {
		return "", false
	}
	for _, capability := range e.bundle.Manifest.Capabilities {
		if capability == api.CapabilityDesktopFiles {
			return e.bundle.Hash, true
		}
	}
	return "", false
}

// SnapshotIdentity scopes an in-memory presentation snapshot to an enabled installation.
// The opaque identity is replaced on disable or installation replacement.
func (m *Manager) SnapshotIdentity(id string) any {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil || e.builtin || e.runtime == nil || !m.isRunnable(e) {
		return nil
	}
	if e.snapshotIdentity == nil {
		e.snapshotIdentity = new(byte)
	}
	return e.snapshotIdentity
}

// MayCopyImage authorizes an action against the same enabled installation that created the view.
// Clipboard History retains access to its own stored pictures without granting arbitrary file access.
func (m *Manager) MayCopyImage(id string, identity any, clipboardPicture bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil || !m.isRunnable(e) {
		return false
	}
	if e.builtin && id == "nevermind.clipboard" && clipboardPicture {
		return true
	}
	if !e.builtin && (e.runtime == nil || identity == nil || identity != e.snapshotIdentity) {
		return false
	}
	for _, capability := range e.bundle.Manifest.Capabilities {
		if capability == api.CapabilityDesktopFiles {
			return true
		}
	}
	return false
}

// ErrChangedInstallation rejects an operation created by another installation.
var ErrChangedInstallation = errors.New("extension installation changed")

func sameInvocationIdentity(actual, expected any) bool {
	if expected != nil && !reflect.TypeOf(expected).Comparable() {
		return false
	}
	return actual == expected
}
