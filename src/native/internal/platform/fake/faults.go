// Package fake provides deterministic test doubles for every port in package platform. Each fake records
// what callers asked for, answers from in-memory state, and can be told to fail the next call of an
// operation with FailNext, so tests check how code recovers. Nothing here reads the real clock, the real
// disk or the network, and nothing sleeps: a test advances time with Clock.Advance and waits for timers
// with Clock.AwaitTimers.
//
// Fakes are bundled by Set, which builds every port in one call:
//
//	set := fake.NewSet()
//	ports := set.Ports()
//	set.Clipboard.WriteText("copied") // simulate the user copying text elsewhere
//	<-ports.Clipboard.Changed()
package fake

import "sync"

// faultSet holds errors queued by FailNext, keyed by operation name. The zero value is ready to use.
type faultSet struct {
	mu   sync.Mutex
	next map[string][]error
	// paths holds errors that fail every call of one operation on one path until cleared.
	paths map[string]error
}

// failPath makes every call of op on name return err until failPath is called again with nil.
func (f *faultSet) failPath(op, name string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := pathFaultKey(op, name)
	if err == nil {
		delete(f.paths, key)
		return
	}
	if f.paths == nil {
		f.paths = map[string]error{}
	}
	f.paths[key] = err
}

// takePath returns the persistent error set for op on name, or nil. Unlike take it does not consume it.
func (f *faultSet) takePath(op, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.paths[pathFaultKey(op, name)]
}

// pathFaultKey joins an operation and its cleaned path into a map key.
func pathFaultKey(op, name string) string {
	return op + " " + cleanKey(name)
}

// failNext queues err for the next call of op. Calls consume queued errors in order.
func (f *faultSet) failNext(op string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.next == nil {
		f.next = map[string][]error{}
	}
	f.next[op] = append(f.next[op], err)
}

// take returns and removes the next queued error for op, or nil when none is queued.
func (f *faultSet) take(op string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	queue := f.next[op]
	if len(queue) == 0 {
		return nil
	}
	err := queue[0]
	f.next[op] = queue[1:]
	return err
}
