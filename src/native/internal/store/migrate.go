package store

import (
	"encoding/json"
	"fmt"
)

// Migration upgrades stored data from schema version From to From+1. Up receives the stored data as raw
// JSON and returns the upgraded JSON. It must be a pure function of its input: it never touches the file,
// which the store backs up and rewrites itself.
type Migration struct {
	From int
	Up   func(data json.RawMessage) (json.RawMessage, error)
}

// runMigrations applies the steps from version from up to version to, in order. Each step's output must be
// valid JSON. Errors wrap ErrMigration and name the failing step.
func runMigrations(migrations []Migration, from, to int, data json.RawMessage) (json.RawMessage, error) {
	current := data
	for version := from; version < to; version++ {
		next, err := migrations[version-1].Up(current)
		if err != nil {
			return nil, fmt.Errorf("%w: step %d to %d: %w", ErrMigration, version, version+1, err)
		}
		if !json.Valid(next) {
			return nil, fmt.Errorf("%w: step %d to %d produced invalid JSON", ErrMigration, version, version+1)
		}
		current = next
	}
	return current, nil
}
