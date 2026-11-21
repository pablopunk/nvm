package manager

import "context"

// ViewActionOrigin is an opaque snapshot of a view action's registered source.
type ViewActionOrigin struct {
	extensionID string
}

// ExtensionID returns the validated built-in extension ID, or empty for other sources.
func (o ViewActionOrigin) ExtensionID() string { return o.extensionID }

// ResolveViewActionOrigin classifies the current registry without discovering actions.
func (m *Manager) ResolveViewActionOrigin(id string) ViewActionOrigin {
	if m == nil {
		return ViewActionOrigin{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.findLocked(id)
	if err != nil || !e.builtin || !validOriginID(e.bundle.Manifest.ID) {
		return ViewActionOrigin{}
	}
	return ViewActionOrigin{extensionID: e.bundle.Manifest.ID}
}

type viewActionRunKey struct{}

type viewActionRunMarker struct {
	marked bool
	origin ViewActionOrigin
}

// WithViewActionRunOrigin marks a host-originated view action with its opaque source.
func WithViewActionRunOrigin(ctx context.Context, origin ViewActionOrigin) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, viewActionRunKey{}, viewActionRunMarker{marked: true, origin: origin})
}

// ConsumeViewActionOrigin shadows the marker only in the returned context.
func ConsumeViewActionOrigin(ctx context.Context) (context.Context, ViewActionOrigin, bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	marker, _ := ctx.Value(viewActionRunKey{}).(viewActionRunMarker)
	return context.WithValue(ctx, viewActionRunKey{}, viewActionRunMarker{}), marker.origin, marker.marked
}
