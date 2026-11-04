package windowmgmt

import (
	"crypto/rand"
	"encoding/hex"
	"slices"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform"
)

const (
	// handleTTL is how long a listed window handle stays usable without being listed again.
	handleTTL = 10 * time.Minute
	// maxHandles caps the registry; the least recently issued handle is dropped first.
	maxHandles = 500
)

// handleTarget is the window a handle stands for. Keys change when an app recreates its window, which
// is what makes a stale handle fail instead of acting on the wrong window.
type handleTarget struct {
	PID      int
	BundleID string
	Key      string
}

type handleEntry struct {
	target    handleTarget
	handle    string
	expiresAt time.Time
}

// handleRegistry maps opaque handles given to the palette and extensions onto windows. Handles are
// random so a caller cannot guess another window's handle, and they expire.
type handleRegistry struct {
	mu      sync.Mutex
	clock   platform.Clock
	newID   func() string
	entries []handleEntry
}

func newHandleRegistry(clock platform.Clock, newID func() string) *handleRegistry {
	if newID == nil {
		newID = randomHandleID
	}
	return &handleRegistry{clock: clock, newID: newID}
}

// issue returns the handle for a window, reusing the existing one while the window is still listed.
func (r *handleRegistry) issue(target handleTarget) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.purgeExpired()
	handle := r.handleFor(target)
	if handle == "" {
		r.evictToCapacity()
		handle = r.newID()
	}
	r.entries = slices.DeleteFunc(r.entries, func(entry handleEntry) bool { return entry.handle == handle })
	r.entries = append(r.entries, handleEntry{target: target, handle: handle, expiresAt: r.clock.Now().Add(handleTTL)})
	return handle
}

// resolve returns the window a handle stands for, or false when it is unknown or expired.
func (r *handleRegistry) resolve(handle string) (handleTarget, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	index := slices.IndexFunc(r.entries, func(entry handleEntry) bool { return entry.handle == handle })
	if index < 0 {
		return handleTarget{}, false
	}
	entry := r.entries[index]
	if !r.clock.Now().Before(entry.expiresAt) {
		r.forgetLocked(handle)
		return handleTarget{}, false
	}
	return entry.target, true
}

func (r *handleRegistry) forget(handle string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.forgetLocked(handle)
}

func (r *handleRegistry) forgetLocked(handle string) {
	r.entries = slices.DeleteFunc(r.entries, func(entry handleEntry) bool { return entry.handle == handle })
}

func (r *handleRegistry) handleFor(target handleTarget) string {
	for _, entry := range r.entries {
		if entry.target == target {
			return entry.handle
		}
	}
	return ""
}

func (r *handleRegistry) purgeExpired() {
	now := r.clock.Now()
	r.entries = slices.DeleteFunc(r.entries, func(entry handleEntry) bool { return !now.Before(entry.expiresAt) })
}

func (r *handleRegistry) evictToCapacity() {
	for len(r.entries) >= maxHandles {
		r.entries = r.entries[1:]
	}
}

func randomHandleID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(raw[:])
}
