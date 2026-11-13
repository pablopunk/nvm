package clipboard

import "time"

// Retention bounds what History keeps. A zero field takes its default from limits.go. A negative MaxAge disables
// expiry.
type Retention struct {
	// MaxEntries caps the entries kept; the oldest go first.
	MaxEntries int
	// MaxAge expires entries copied longer ago than this.
	MaxAge time.Duration
	// MaxImageBytes caps the PNG bytes of image entries.
	MaxImageBytes int64
	// MaxMemoryBytes caps the text and markup held in memory by entries.
	MaxMemoryBytes int64
}

// withDefaults fills every zero field with its default from limits.go.
func (r Retention) withDefaults() Retention {
	if r.MaxEntries <= 0 {
		r.MaxEntries = DefaultMaxEntries
	}
	if r.MaxAge == 0 {
		r.MaxAge = DefaultMaxAge
	}
	if r.MaxImageBytes <= 0 {
		r.MaxImageBytes = DefaultMaxImageBytes
	}
	if r.MaxMemoryBytes <= 0 {
		r.MaxMemoryBytes = DefaultMaxMemoryBytes
	}
	return r
}
