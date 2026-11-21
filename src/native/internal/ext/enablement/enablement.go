// Package enablement records which extensions the user has turned on. Dropped-in extensions default to off:
// a missing entry means disabled.
//
// The user's choice lives in the synced config folder (config.json, under "extensions"), so enabling follows the
// extensions folder from machine to machine. Each entry is pinned to the capabilities the user reviewed: an
// extension that now asks for more is off until reviewed again. The app's data folder keeps the older local
// document, which is read once to migrate its choices and is still written for builds that predate the synced
// file. Without a synced store the package behaves as it did before and keeps everything locally.
//
// Layer: ext. The package imports store, portable and the standard library only.
package enablement

import (
	"github.com/pablopunk/nvm/native/internal/portable"
	"github.com/pablopunk/nvm/native/internal/store"
)

// documentName is the file in the app's data directory that holds the enabled set.
const documentName = "extension-enabled.json"

// State is the stored document. Only enabled extensions appear; disabling removes the entry.
type State struct {
	Enabled map[string]bool `json:"enabled"`
	// Inherited maps each extension id that took its state from a 1.x file to that file's name. An id is here once.
	Inherited map[string]string `json:"inherited,omitempty"`
	// Migrated lists the ids whose local choice already moved to the synced file, so a later disable made on another
	// machine is not undone by the old local entry.
	Migrated map[string]bool `json:"migrated,omitempty"`
}

// Set reads and writes the enabled state of extensions.
type Set struct {
	doc    *store.Collection[State]
	synced *portable.Store
}

// Open attaches the local enabled-state document to s, with no synced file. It must be called once per store.
func Open(s *store.Store) *Set {
	return &Set{doc: store.Doc(s, documentName, 1, func() State { return State{Enabled: map[string]bool{}} })}
}

// OpenSynced attaches the local document to s and the synced choices to config, the shared config.json. Existing
// local choices move to config the first time each extension is read.
func OpenSynced(s *store.Store, config *portable.Store) *Set {
	set := Open(s)
	set.synced = config
	return set
}

// Enabled reports whether id is turned on for the capabilities it declares now. A load error is returned with
// false, so a damaged file never turns an extension on by accident.
func (s *Set) Enabled(id string, capabilities []string) (bool, error) {
	state, err := s.doc.Load()
	if err != nil {
		return false, err
	}
	if s.synced == nil {
		return state.Enabled[id], nil
	}
	return s.syncedEnabled(state, id, capabilities)
}

// SetEnabled turns id on or off and saves it. Turning on pins the choice to capabilities, the set the user reviewed.
func (s *Set) SetEnabled(id string, enabled bool, capabilities []string) error {
	if s.synced != nil {
		if err := s.writeSynced(id, enabled, capabilities); err != nil {
			return err
		}
	}
	return s.doc.Update(func(state *State) {
		if state.Enabled == nil {
			state.Enabled = map[string]bool{}
		}
		if enabled {
			state.Enabled[id] = true
			s.markMigrated(state, id)
			return
		}
		delete(state.Enabled, id)
		s.markMigrated(state, id)
	})
}

func (s *Set) markMigrated(state *State, id string) {
	if s.synced == nil {
		return
	}
	if state.Migrated == nil {
		state.Migrated = map[string]bool{}
	}
	state.Migrated[id] = true
}

// Inherited reports whether id already took its state from a 1.x file. Only the first scan of a converted folder
// does, so a later change made here is never overwritten.
func (s *Set) Inherited(id string) (bool, error) {
	state, err := s.doc.Load()
	if err != nil {
		return false, err
	}
	if _, done := state.Inherited[id]; done {
		return true, nil
	}
	if s.synced == nil {
		return false, nil
	}
	cfg, err := s.synced.Load()
	if err != nil {
		return false, err
	}
	_, done := cfg.String(configKey, "inherited", id)
	return done, nil
}

// Inherit records that id took its state from the 1.x file convertedFrom, and turns id on when that file was on. A
// state already recorded for id is kept, and so is an id that is on already.
func (s *Set) Inherit(id, convertedFrom string, on bool, capabilities []string) error {
	done, err := s.Inherited(id)
	if err != nil || done {
		return err
	}
	if s.synced != nil {
		return s.inheritSynced(id, convertedFrom, on, capabilities)
	}
	return s.doc.Update(func(state *State) {
		if state.Enabled == nil {
			state.Enabled = map[string]bool{}
		}
		if state.Inherited == nil {
			state.Inherited = map[string]string{}
		}
		state.Inherited[id] = convertedFrom
		if on {
			state.Enabled[id] = true
		}
	})
}
