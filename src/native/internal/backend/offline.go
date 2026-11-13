package backend

import "sync"

// offlineThreshold is how many network failures in a row flip the client to offline.
const offlineThreshold = 3

// offlineTracker counts consecutive network failures. Any response from the backend, even an error status,
// proves the network works and clears the count.
type offlineTracker struct {
	mu       sync.Mutex
	failures int
	offline  bool
}

// unreachable records one network failure. It reports true when this failure flipped the client to offline.
func (t *offlineTracker) unreachable() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failures++
	if t.offline || t.failures < offlineThreshold {
		return false
	}
	t.offline = true
	return true
}

// reachable records a response. It reports true when this response flipped the client back to online.
func (t *offlineTracker) reachable() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failures = 0
	if !t.offline {
		return false
	}
	t.offline = false
	return true
}

// isOffline reports the current state.
func (t *offlineTracker) isOffline() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.offline
}
