// Package enablement records which extensions the user has turned on. Dropped-in extensions default to off:
// a missing entry means disabled. The state lives in one store document, so writes are atomic and versioned
// like every other Nevermind file.
//
// Layer: ext. The package imports store and the standard library only.
package enablement

import "github.com/pablopunk/nvm/native/internal/store"

// documentName is the file in the app's data directory that holds the enabled set.
const documentName = "extension-enabled.json"

// State is the stored document. Only enabled extensions appear; disabling removes the entry.
type State struct {
	Enabled map[string]bool `json:"enabled"`
	// Inherited maps each extension id that took its state from a 1.x file to that file's name. An id is here once.
	Inherited map[string]string `json:"inherited,omitempty"`
}

// Set reads and writes the enabled state of extensions.
type Set struct {
	doc *store.Collection[State]
}

// Open attaches the enabled-state document to s. It must be called once per store.
func Open(s *store.Store) *Set {
	return &Set{doc: store.Doc(s, documentName, 1, func() State { return State{Enabled: map[string]bool{}} })}
}

// Enabled reports whether id is turned on. A load error is returned with false, so a damaged file never
// turns an extension on by accident.
func (s *Set) Enabled(id string) (bool, error) {
	state, err := s.doc.Load()
	if err != nil {
		return false, err
	}
	return state.Enabled[id], nil
}

// SetEnabled turns id on or off and saves the document.
func (s *Set) SetEnabled(id string, enabled bool) error {
	return s.doc.Update(func(state *State) {
		if state.Enabled == nil {
			state.Enabled = map[string]bool{}
		}
		if enabled {
			state.Enabled[id] = true
			return
		}
		delete(state.Enabled, id)
	})
}

// Inherited reports whether id already took its state from a 1.x file. Only the first scan of a converted folder
// does, so a later change made here is never overwritten.
func (s *Set) Inherited(id string) (bool, error) {
	state, err := s.doc.Load()
	if err != nil {
		return false, err
	}
	_, done := state.Inherited[id]
	return done, nil
}

// Inherit records that id took its state from the 1.x file convertedFrom, and turns id on when that file was on. A
// state already recorded for id is kept, and so is an id that is on already.
func (s *Set) Inherit(id, convertedFrom string, on bool) error {
	return s.doc.Update(func(state *State) {
		if state.Enabled == nil {
			state.Enabled = map[string]bool{}
		}
		if state.Inherited == nil {
			state.Inherited = map[string]string{}
		}
		if _, done := state.Inherited[id]; done {
			return
		}
		state.Inherited[id] = convertedFrom
		if on {
			state.Enabled[id] = true
		}
	})
}
