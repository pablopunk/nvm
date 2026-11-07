package manager

import (
	"github.com/pablopunk/nvm/native/internal/ext/sourcecheck"
)

// CatalogEntry is one installed folder extension as the extension builder sees it: its identity, whether it is on, and
// the command and root row titles its source declares. The titles are read from the source, so a disabled extension
// is listed with the same titles as a running one.
type CatalogEntry struct {
	ID       string
	Title    string
	Subtitle string
	Enabled  bool
	Titles   []string
}

// Catalog returns one entry per folder extension, sorted by folder name. Built-in extensions are not listed.
func (m *Manager) Catalog() []CatalogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries := make([]CatalogEntry, 0, len(m.entries))
	for _, key := range m.sortedKeysLocked() {
		e := m.entries[key]
		if e.builtin {
			continue
		}
		manifest := e.bundle.Manifest
		entries = append(entries, CatalogEntry{
			ID:       manifest.ID,
			Title:    manifest.Title,
			Subtitle: manifest.Subtitle,
			Enabled:  m.isEnabled(manifest.ID),
			Titles:   sourcecheck.DisplayTitles(map[string][]byte{"main.go": e.bundle.Source}),
		})
	}
	return entries
}
