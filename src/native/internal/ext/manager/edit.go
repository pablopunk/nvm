package manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/install"
)

// EditRevision is an opaque manager-issued admission token. Its contents are never sent to a model.
type EditRevision struct {
	manager    *Manager
	id         string
	generation uint64
	exists     bool
	files      map[string][]byte
}

var ErrEditConflict = errors.New("extension changed since it was read")

// EditSnapshot captures all replaceable source files or confirms the target is absent. Unknown or
// unreadable folders are errors. A revision expires after any manager installation lifecycle change.
func (m *Manager) EditSnapshot(id string) (map[string][]byte, *EditRevision, error) {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\`) {
		return nil, nil, errors.New("invalid extension folder name")
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, nil, errors.New("extension manager is closed")
	}
	generation := m.editGeneration
	if e, err := m.findLocked(id); err == nil && (e.builtin || filepath.Base(e.bundle.Dir) != id) {
		m.mu.Unlock()
		return nil, nil, errors.New("extension is not an editable folder with this id")
	}
	m.mu.Unlock()
	files, exists, err := m.readEditTarget(id)
	if err != nil {
		return nil, nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.editGeneration != generation {
		return nil, nil, ErrEditConflict
	}
	if e, err := m.findLocked(id); err == nil && (e.builtin || filepath.Base(e.bundle.Dir) != id || !exists) {
		return nil, nil, ErrEditConflict
	}
	return cloneEditFiles(files), &EditRevision{manager: m, id: id, generation: generation, exists: exists, files: files}, nil
}

func (m *Manager) readEditTarget(id string) (map[string][]byte, bool, error) {
	_, err := m.ports.Files.Lstat(filepath.Join(m.root, id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	files, err := install.SourceSnapshot(m.ports.Files, m.root, id)
	if err != nil {
		return nil, false, err
	}
	return files, true, nil
}

// InstallEdit admits an already validated candidate only if its manager-issued revision remains current.
// Cancellation is checked immediately before publication. Once admitted, errors may follow visible effects.
// Manager operations are serialized; an uncooperative external writer is not locked out.
func (m *Manager) InstallEdit(ctx context.Context, id string, contents map[string][]byte, enable bool, revision *EditRevision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || revision == nil || revision.manager != m || revision.id != id || revision.generation != m.editGeneration {
		return ErrEditConflict
	}
	current, exists, err := m.readEditTarget(id)
	if err != nil {
		return fmt.Errorf("revalidate extension: %w", err)
	}
	if exists != revision.exists || !equalEditFiles(current, revision.files) {
		return ErrEditConflict
	}
	if e, err := m.findLocked(id); err == nil && (e.builtin || filepath.Base(e.bundle.Dir) != id || !exists) {
		return ErrEditConflict
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.editGeneration++
	if err := install.InstallContext(ctx, m.ports.Files, m.ports.Clock, m.root, id, contents); err != nil {
		return err
	}
	if err := m.enabled.SetEnabled(id, enable); err != nil {
		return fmt.Errorf("save enabled state: %w", err)
	}
	return m.reloadLocked()
}

func cloneEditFiles(files map[string][]byte) map[string][]byte {
	if files == nil {
		return nil
	}
	copy := make(map[string][]byte, len(files))
	for name, data := range files {
		copy[name] = bytes.Clone(data)
	}
	return copy
}

func equalEditFiles(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for name, data := range a {
		other, ok := b[name]
		if !ok || !bytes.Equal(data, other) {
			return false
		}
	}
	return true
}
