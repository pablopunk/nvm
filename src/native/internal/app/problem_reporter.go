package app

import "sync"

// problemReporter lets a repeating problem be shown once: the same message is not reported again until a run without
// a problem has passed.
type problemReporter struct {
	mu   sync.Mutex
	last string
}

// shouldReport records message and reports whether it is new. An empty message means no problem and resets the memory.
func (r *problemReporter) shouldReport(message string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	fresh := message != "" && message != r.last
	r.last = message
	return fresh
}
