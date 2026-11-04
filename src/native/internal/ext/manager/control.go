package manager

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/install"
	"github.com/pablopunk/nvm/native/internal/ext/supervisor"
)

// ErrUnknownExtension is returned for an id that is not on disk.
var ErrUnknownExtension = errors.New("no extension with that id is installed")

// Listing is what the extension manager shows for one folder: its declared manifest, whether it is on, what its
// runtime is doing, and why it cannot run when it cannot.
type Listing struct {
	ID                  string
	Title               string
	Subtitle            string
	Dir                 string
	Enabled             bool
	Problems            []string
	Capabilities        []api.Capability
	UnknownCapabilities []api.Capability
	Runtime             supervisor.Status
}

// List returns one listing per extension folder, sorted by folder name.
func (m *Manager) List() []Listing {
	m.mu.Lock()
	defer m.mu.Unlock()
	listings := make([]Listing, 0, len(m.entries))
	for _, key := range m.sortedKeysLocked() {
		listings = append(listings, m.listingLocked(key, m.entries[key]))
	}
	return listings
}

func (m *Manager) listingLocked(key string, e *entry) Listing {
	manifest := e.bundle.Manifest
	listing := Listing{
		ID:                  manifest.ID,
		Title:               manifest.Title,
		Subtitle:            manifest.Subtitle,
		Dir:                 e.bundle.Dir,
		Enabled:             m.isEnabled(manifest.ID),
		Problems:            e.bundle.Problems,
		Capabilities:        manifest.Capabilities,
		UnknownCapabilities: manifest.UnknownCapabilities(),
	}
	if e.runtime != nil {
		listing.Runtime = e.runtime.Status()
	}
	if listing.ID == "" {
		listing.ID = key
	}
	return listing
}

// Source returns the source text of an installed extension, for the source view.
func (m *Manager) Source(id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil {
		return "", err
	}
	return string(e.bundle.Source), nil
}

// Definition returns the manifest and source of an installed extension: the version an AI chat edits.
func (m *Manager) Definition(id string) (api.Manifest, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil {
		return api.Manifest{}, "", err
	}
	return e.bundle.Manifest, string(e.bundle.Source), nil
}

// Enable turns an extension on and starts it. A disabled-by-failure extension is reset first.
func (m *Manager) Enable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil {
		return err
	}
	if err := m.enabled.SetEnabled(id, true); err != nil {
		return fmt.Errorf("save enabled state: %w", err)
	}
	if e.runtime == nil {
		return fmt.Errorf("extension %s does not load: %s", id, joinProblems(e.bundle.Problems))
	}
	e.runtime.Reset()
	e.runtime.Start()
	return nil
}

// Disable turns an extension off and stops its child process.
func (m *Manager) Disable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil {
		return err
	}
	if err := m.enabled.SetEnabled(id, false); err != nil {
		return fmt.Errorf("save enabled state: %w", err)
	}
	stopEntry(e)
	return nil
}

// Install writes an extension folder atomically and reloads it. The enabled state is set to enable, so a new
// version of an enabled extension is off until its owner reviews it again: dropped-in files default to off.
func (m *Manager) Install(id string, contents map[string][]byte, enable bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := install.Install(m.ports.Files, m.ports.Clock, m.root, id, contents); err != nil {
		return err
	}
	if err := m.enabled.SetEnabled(id, enable); err != nil {
		return fmt.Errorf("save enabled state: %w", err)
	}
	return m.reloadLocked()
}

// Check runs the candidate files the way a child would load them, without publishing anything. It returns the
// protocol methods the source implements, or why it does not load, including the Yaegi compile errors. The
// staging folder is removed on every path.
func (m *Manager) Check(id string, contents map[string][]byte) ([]string, error) {
	staging, staged, err := install.Stage(m.ports.Files, m.ports.Clock, m.root, id, contents)
	if err != nil {
		return nil, err
	}
	defer func() { _ = install.Discard(m.ports.Files, staging) }()
	identity := api.Identity{ID: id, Title: staged.Manifest.Title}
	return supervisor.ProbeSource(m.config, identity, filepath.Join(staging, bundle.SourceFile))
}

// Uninstall stops the extension, removes its folder and forgets its enabled state.
func (m *Manager) Uninstall(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil {
		return err
	}
	stopEntry(e)
	if err := install.Uninstall(m.ports.Files, m.ports.Clock, m.root, filepath.Base(e.bundle.Dir)); err != nil {
		return err
	}
	if err := m.enabled.SetEnabled(id, false); err != nil {
		return fmt.Errorf("save enabled state: %w", err)
	}
	return m.reloadLocked()
}

// reloadLocked rescans the folder and applies it. A scan error changes nothing, so a failed read never looks
// like an uninstall of every extension.
func (m *Manager) reloadLocked() error {
	bundles, err := bundle.Scan(m.ports.Files, m.root)
	if err != nil {
		return err
	}
	m.refreshLocked(bundles)
	return nil
}

// findLocked returns the entry whose manifest id, or folder name, is id.
func (m *Manager) findLocked(id string) (*entry, error) {
	if e, ok := m.entries[id]; ok {
		return e, nil
	}
	for _, e := range m.entries {
		if e.bundle.Manifest.ID == id {
			return e, nil
		}
	}
	return nil, ErrUnknownExtension
}

func joinProblems(problems []string) string {
	return strings.Join(problems, "; ")
}
