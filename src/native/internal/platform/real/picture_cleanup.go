package real

import "sync"

type pictureCleanup struct {
	mu      sync.Mutex
	budget  *pictureStagingBudget
	path    string
	remove  func(string) error
	release func()
	done    bool
}

// cleanup retains failed removals, including failures before staging returns a usable source.
func (b *pictureStagingBudget) cleanup(path string, remove func(string) error, release func()) func() {
	entry := &pictureCleanup{budget: b, path: path, remove: remove, release: release}
	return entry.run
}

func (e *pictureCleanup) run() {
	_ = e.attempt()
}

func (e *pictureCleanup) attempt() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.done {
		return nil
	}
	if err := e.remove(e.path); err != nil {
		e.budget.mu.Lock()
		if e.budget.pending == nil {
			e.budget.pending = make(map[*pictureCleanup]struct{})
		}
		if _, alreadyQueued := e.budget.pending[e]; !alreadyQueued {
			e.budget.pending[e] = struct{}{}
			close(e.budget.changed)
			e.budget.changed = make(chan struct{})
		}
		e.budget.mu.Unlock()
		return err
	}
	e.done = true
	e.budget.mu.Lock()
	delete(e.budget.pending, e)
	e.budget.mu.Unlock()
	e.release()
	return nil
}

func (b *pictureStagingBudget) retryCleanups() {
	b.mu.Lock()
	entries := make([]*pictureCleanup, 0, len(b.pending))
	for entry := range b.pending {
		entries = append(entries, entry)
	}
	b.mu.Unlock()
	for _, entry := range entries {
		entry.run()
	}
}
