package windowmgmt

import (
	"log"
	"sync"

	"github.com/pablopunk/nvm/native/internal/store"
	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// SettingsDocument is the window management settings file. Version 1 holds the geometry settings as they are.
type SettingsDocument struct {
	Settings geometry.Settings
}

// DefaultSettingsDocument is the document a fresh install starts from.
func DefaultSettingsDocument() SettingsDocument {
	return SettingsDocument{Settings: geometry.DefaultSettings()}
}

// SettingsStore is what the window commands read. Changes persist through the document when the directory is
// writable and stay in memory otherwise, so a read-only instance still honors the settings it was given.
type SettingsStore struct {
	mu      sync.Mutex
	doc     *store.Collection[SettingsDocument]
	current geometry.Settings
}

// NewSettingsStore loads the saved settings. A nil document keeps the settings in memory only.
func NewSettingsStore(doc *store.Collection[SettingsDocument]) *SettingsStore {
	s := &SettingsStore{doc: doc, current: geometry.DefaultSettings()}
	if doc == nil {
		return s
	}
	saved, err := doc.Load()
	if err != nil {
		log.Println("window management settings: using defaults:", err)
		return s
	}
	s.current = saved.Settings
	return s
}

// Current returns the settings the next command will use.
func (s *SettingsStore) Current() geometry.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// Update applies change to a copy of the settings, keeps the result, and saves it. A save error is returned
// after the change is in effect for this session.
func (s *SettingsStore) Update(change func(*geometry.Settings)) error {
	s.mu.Lock()
	next := s.current
	change(&next)
	s.current = next
	s.mu.Unlock()
	if s.doc == nil {
		return nil
	}
	return s.doc.Update(func(document *SettingsDocument) { document.Settings = next })
}

// Reset restores every setting to its default.
func (s *SettingsStore) Reset() error {
	return s.Update(func(settings *geometry.Settings) { *settings = geometry.DefaultSettings() })
}
