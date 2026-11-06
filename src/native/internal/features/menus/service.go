package menus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

var ErrInvalidHandle = errors.New("menu item is unavailable or stale")

// Item contains only data safe to present outside the host.
type Item struct {
	Handle   string
	Title    string
	Path     []string
	Shortcut string
	Enabled  bool
	Checked  bool
}

// SnapshotState is the current source's cached data and most recent refresh error.
type SnapshotState struct {
	Items []Item
	Err   error
}

type target struct {
	source    platform.MenuSource
	reference string
	enabled   bool
}

// Service owns handles for one captured source. Refresh is asynchronous; a
// successful refresh replaces all handles, while failure preserves the snapshot.
type Service struct {
	mu         sync.Mutex
	port       platform.CurrentAppMenus
	source     platform.MenuSource
	generation uint64
	cancel     context.CancelFunc
	items      []Item
	lastError  error
	targets    map[string]target
}

func New(port platform.CurrentAppMenus) *Service { return &Service{port: port} }

// Capture invalidates the previous source and its handles, including in-flight reads.
func (s *Service) Capture(source platform.MenuSource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidate()
	s.source = source
}

func (s *Service) invalidate() {
	s.generation++
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.items = nil
	s.lastError = nil
	s.targets = nil
}

// Dismiss ends the handle lifetime.
func (s *Service) Dismiss() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidate()
	s.source = platform.MenuSource{}
}

// Snapshot returns an independent copy immediately, including during refresh.
func (s *Service) Snapshot() []Item {
	return s.State().Items
}

// State returns an independent, consistent snapshot of the current source.
func (s *Service) State() SnapshotState {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := append([]Item(nil), s.items...)
	for i := range items {
		items[i].Path = append([]string(nil), items[i].Path...)
	}
	return SnapshotState{Items: items, Err: s.lastError}
}

// Refresh starts one bounded read, canceling and superseding any previous read.
// changed is a wake-up notification, not a refresh result. It may arrive after a
// later source change; consumers read State when they handle the notification.
func (s *Service) Refresh(ctx context.Context, budget time.Duration, changed func()) {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.generation++
	generation, source := s.generation, s.source
	readCtx, cancel := context.WithTimeout(ctx, budget)
	s.cancel = cancel
	s.mu.Unlock()
	go func() {
		defer cancel()
		var entries []platform.MenuEntry
		var err error
		if s.port == nil || source.PID <= 0 || source.Identity == "" {
			err = ErrInvalidHandle
		} else {
			entries, err = s.port.Snapshot(readCtx, source)
		}
		if err == nil {
			err = readCtx.Err()
		}
		items := make([]Item, 0, len(entries))
		targets := make(map[string]target, len(entries))
		if err == nil {
			for _, entry := range entries {
				var token [16]byte
				if _, err = rand.Read(token[:]); err != nil {
					break
				}
				handle := hex.EncodeToString(token[:])
				items = append(items, Item{handle, entry.Title, append([]string(nil), entry.Path...), entry.Shortcut, entry.Enabled, entry.Checked})
				targets[handle] = target{source, entry.Reference, entry.Enabled}
			}
		}
		s.mu.Lock()
		if generation != s.generation {
			s.mu.Unlock()
			return
		}
		s.cancel = nil
		s.lastError = err
		if err == nil {
			s.items, s.targets = items, targets
		}
		s.mu.Unlock()
		if changed != nil {
			changed()
		}
	}()
}

// Invoke resolves only a current, enabled host-issued handle. The adapter must
// independently revalidate source and item immediately before the OS action. A valid
// invocation claims its target at operation start; later capture or dismissal does
// not wait for or revoke an already claimed operation.
func (s *Service) Invoke(ctx context.Context, handle string) error {
	s.mu.Lock()
	t, ok := s.targets[handle]
	port := s.port
	valid := ok && t.enabled && t.source == s.source && port != nil
	s.mu.Unlock()
	if !valid {
		return ErrInvalidHandle
	}
	return port.Invoke(ctx, t.source, t.reference)
}
