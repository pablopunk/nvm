package store

import (
	"io/fs"
	"time"
)

const (
	defaultDebounce    = 500 * time.Millisecond
	defaultKeepBackups = 3
	defaultFileMode    = fs.FileMode(0o600)
)

// Option configures a Store. Options apply to every document of the store.
type Option func(*settings)

// settings is the resolved configuration of a store.
type settings struct {
	debounce    time.Duration
	keepBackups int
	fileMode    fs.FileMode
	lease       *Lease
	onSaved     func(path string)
}

// defaultSettings returns the configuration used when no option is given.
func defaultSettings() settings {
	return settings{debounce: defaultDebounce, keepBackups: defaultKeepBackups, fileMode: defaultFileMode}
}

// WithDebounce sets how long a document waits after its last Update before it is written. Zero writes on
// the next scheduler turn. The default is 500ms.
func WithDebounce(d time.Duration) Option {
	return func(s *settings) { s.debounce = max(d, 0) }
}

// WithKeepBackups keeps the last n earlier versions as name.json.bak.1 (newest) through name.json.bak.n.
// Zero keeps none. The default is 3.
func WithKeepBackups(n int) Option {
	return func(s *settings) { s.keepBackups = max(n, 0) }
}

// WithFileMode sets the permission bits of written files. The default 0600 suits anything private; use
// 0644 only for files that hold nothing private.
func WithFileMode(mode fs.FileMode) Option {
	return func(s *settings) { s.fileMode = mode.Perm() }
}

// WithLease ties writes to a held lease. Every write verifies the lease first and fails with ErrLeaseLost
// once another instance has taken it over.
func WithLease(lease *Lease) Option {
	return func(s *settings) { s.lease = lease }
}

// WithOnSaved calls fn with the path of each file the store writes. fn runs while the document's lock is
// held, so it must not call back into the store.
func WithOnSaved(fn func(path string)) Option {
	return func(s *settings) { s.onSaved = fn }
}
