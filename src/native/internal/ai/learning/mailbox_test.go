package learning

import (
	"sync"
	"testing"
)

type heldMailboxSink struct {
	entered chan struct{}
	release chan struct{}
	mu      sync.Mutex
	seen    []string
}

func (s *heldMailboxSink) Message(_, _, _ string) {
	s.mu.Lock()
	s.seen = append(s.seen, "message")
	s.mu.Unlock()
}
func (s *heldMailboxSink) Outcome(_, _ string) {}
func (s *heldMailboxSink) Exit(_ string, busy bool) {
	if busy {
		close(s.entered)
		<-s.release
	}
	s.mu.Lock()
	s.seen = append(s.seen, "exit")
	s.mu.Unlock()
}
func (s *heldMailboxSink) Settled(_ string) {
	s.mu.Lock()
	s.seen = append(s.seen, "settled")
	s.mu.Unlock()
}
func (s *heldMailboxSink) Remove(_ string) {
	s.mu.Lock()
	s.seen = append(s.seen, "remove")
	s.mu.Unlock()
}
func TestMailboxEnqueueDoesNotWaitForBusySink(t *testing.T) {
	sink := &heldMailboxSink{entered: make(chan struct{}), release: make(chan struct{})}
	mailbox := NewMailbox(sink)
	defer mailbox.Close()
	mailbox.Exit("one", true)
	<-sink.entered
	mailbox.Capture("two", []Message{{Role: "user", Text: "Prefer short labels"}, {Role: "user", Text: "password is canary"}}, nil)
	mailbox.Exit("two", false)
	mailbox.Settled("two")
	mailbox.Remove("two")
	mailbox.mu.Lock()
	event := mailbox.items["two"]
	if event == nil || !event.removed || len(event.messages) != 0 {
		t.Fatal("removal did not discard queued capture")
	}
	mailbox.mu.Unlock()
	close(sink.release)
}
