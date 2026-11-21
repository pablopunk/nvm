package safego

import "sync"

// Watch counts the goroutines started with Go or Group that have not returned yet, by name. Leak tests
// compare counts before and after; diagnostics can show the same numbers. Names are free-form labels,
// so give each call site a stable name.
type Watch struct {
	mu   sync.Mutex
	live map[string]int
}

var watching = &Watch{live: map[string]int{}}

// Watching returns the process-wide Watch that every Go and Group goroutine reports to.
func Watching() *Watch {
	return watching
}

// Live returns how many goroutines started under name are still running.
func (w *Watch) Live(name string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.live[name]
}

// Total returns how many goroutines started by this package are still running, under any name.
func (w *Watch) Total() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	total := 0
	for _, count := range w.live {
		total += count
	}
	return total
}

func (w *Watch) add(name string, delta int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.live[name] += delta
	if w.live[name] == 0 {
		delete(w.live, name)
	}
}

// track runs body and keeps the name counted for as long as it runs.
func track(name string, body func()) {
	watching.add(name, 1)
	defer watching.add(name, -1)
	body()
}
