package jobs

import (
	"github.com/pablopunk/nvm/native/internal/store"
)

// stateSchemaVersion is the version of the jobs document this build writes.
const stateSchemaVersion = 1

// State is the persisted document: what the user chose and what each job did. Run history is kept per job, so a
// restart shows the same failures and the enabled jobs stay as the user left them.
type State struct {
	Jobs map[string]JobState `json:"jobs"`
}

// JobState is the persisted part of one job. Enabled is nil until the user changes the job, so the definition's
// default applies. Halted is the reason the scheduler stopped the job; only enabling it again clears the reason.
type JobState struct {
	Enabled  *bool          `json:"enabled,omitempty"`
	Halted   string         `json:"halted,omitempty"`
	Runs     int            `json:"runs,omitempty"`
	Failures int            `json:"failures,omitempty"`
	History  []HistoryEntry `json:"history,omitempty"`
}

// StateDocument registers the jobs document, jobs.json, in s. Pass the result to Config.State.
func StateDocument(s *store.Store) *store.Collection[State] {
	return store.Doc(s, "jobs.json", stateSchemaVersion, defaultState)
}

func defaultState() State {
	return State{Jobs: map[string]JobState{}}
}

// loadState returns the saved state, or the defaults when there is no document or it cannot be read.
func loadState(doc *store.Collection[State]) State {
	if doc == nil {
		return defaultState()
	}
	state, err := doc.Load()
	if err != nil || state.Jobs == nil {
		return defaultState()
	}
	return state
}
