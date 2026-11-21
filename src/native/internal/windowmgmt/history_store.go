package windowmgmt

import (
	"sync"

	"github.com/pablopunk/nvm/native/internal/windowmgmt/geometry"
)

// historyStore is the engine's per-window memory: the restore frame and the last layout. Commands for
// different processes run in parallel, so the store is guarded.
type historyStore struct {
	mu     sync.Mutex
	memory *geometry.Memory
}

func newHistoryStore() *historyStore {
	return &historyStore{memory: geometry.NewMemory()}
}

// read returns a copy of the window's history, or nil when Nevermind has none for it.
func (s *historyStore) read(windowKey string) *geometry.History {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.memory.Read(windowKey)
	if !ok {
		return nil
	}
	return &entry
}

func (s *historyStore) write(windowKey string, entry geometry.History) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memory.Write(windowKey, entry)
}

func (s *historyStore) forget(windowKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memory.Forget(windowKey)
}

// dropProcess forgets every window of a process that is gone.
func (s *historyStore) dropProcess(pid int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memory.DropProcess(pid)
}
