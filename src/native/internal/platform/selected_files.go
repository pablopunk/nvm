package platform

import "context"

// SelectedFiles reads file paths selected in the application that owned the captured focus target.
// An application without file selection returns an empty slice. A missing or changed target returns
// ErrNotFound; denied OS automation returns ErrDenied; unsupported systems return ErrUnsupported.
type SelectedFiles interface {
	Paths(ctx context.Context, target FocusTarget) ([]string, error)
}
