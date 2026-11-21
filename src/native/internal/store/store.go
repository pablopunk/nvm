package store

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// noticeBuffer bounds the notices nobody has read yet. The UI drains them promptly; when the buffer is
// full the newest notice is dropped rather than blocking a write.
const noticeBuffer = 64

// Store owns one directory of documents. It is created with Open, filled with Doc, and shut down with
// Close. A Store is safe for concurrent use.
type Store struct {
	files   platform.FS
	clock   platform.Clock
	dir     string
	cfg     settings
	mu      sync.Mutex
	closed  bool
	names   map[string]struct{}
	docs    []document
	notices chan Notice
	done    chan struct{}
}

// document is what the store needs from a Collection regardless of its type parameter.
type document interface {
	flushNow() error
	closeDocument() error
}

// Open prepares dir for documents, creating it when missing. Files are read lazily, when a document is
// first loaded. Open does not take the lease: acquire it with AcquireLease and pass it with WithLease.
func Open(files platform.FS, clock platform.Clock, dir string, opts ...Option) (*Store, error) {
	if dir == "" {
		return nil, errors.New("store: open needs a directory")
	}
	cfg := defaultSettings()
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := files.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("store: create directory: %w", err)
	}
	return &Store{
		files:   files,
		clock:   clock,
		dir:     dir,
		cfg:     cfg,
		names:   map[string]struct{}{},
		notices: make(chan Notice, noticeBuffer),
		done:    make(chan struct{}),
	}, nil
}

// Notices returns the channel the UI reads to show quarantine, read-only and save notices without
// blocking. The channel is closed by Close after the final flush.
func (s *Store) Notices() <-chan Notice {
	return s.notices
}

// Flush writes every document that has unsaved changes. Quit handlers call it; it succeeds without writing
// when nothing changed. Errors from each document are joined.
func (s *Store) Flush() error {
	s.mu.Lock()
	docs := slices.Clone(s.docs)
	s.mu.Unlock()
	var errs []error
	for _, document := range docs {
		if err := document.flushNow(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Close flushes every document, stops the background writers, closes subscriptions and the notice
// channel, and then releases the lease passed with WithLease. Writes after Close return ErrClosed.
func (s *Store) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.done)
	docs := slices.Clone(s.docs)
	s.mu.Unlock()

	var errs []error
	for _, document := range docs {
		if err := document.closeDocument(); err != nil {
			errs = append(errs, err)
		}
	}

	s.mu.Lock()
	close(s.notices)
	s.mu.Unlock()
	if s.cfg.lease != nil {
		if err := s.cfg.lease.Release(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// InstallCrashFlush returns a function that writes every document with unsaved changes, ignoring errors.
// Call it from the app's signal and panic handlers; this package installs no signal handler itself. The
// function takes each document's lock, so it waits for an in-flight write to finish. Locks held by a
// panicking goroutine are released when its deferred unlocks run during the unwind.
func (s *Store) InstallCrashFlush() func() {
	return func() {
		_ = s.Flush()
	}
}

// attach registers a document under its file name. Two documents with one name would overwrite each
// other, so it panics.
func (s *Store) attach(name string, document document) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, taken := s.names[name]; taken {
		panic("store: two documents named " + name)
	}
	s.names[name] = struct{}{}
	s.docs = append(s.docs, document)
}

// isClosed reports whether Close has started.
func (s *Store) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// notify queues a notice without blocking. Notices after Close are dropped.
func (s *Store) notify(notice Notice) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.notices <- notice:
	default:
	}
}

// verifyLease checks the lease the store was opened with before a write. Without a lease it passes.
func (s *Store) verifyLease() error {
	if s.cfg.lease == nil {
		return nil
	}
	return s.cfg.lease.Verify()
}

// savedHook reports a written file to the WithOnSaved callback, if one was given.
func (s *Store) savedHook(path string) {
	if s.cfg.onSaved != nil {
		s.cfg.onSaved(path)
	}
}
