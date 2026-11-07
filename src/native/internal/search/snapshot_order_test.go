package search

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestSnapshotPublicationDropsOlderRevisionsWithinTheSameQuery(t *testing.T) {
	var delivered []Snapshot
	c := NewCoordinator(Options{Clock: fake.NewClock(), Synchronous: true, OnSnapshot: func(s Snapshot) { delivered = append(delivered, s) }})
	c.Query("", nil)
	older := c.current(c.Gen())
	newer := c.current(c.Gen())
	c.publish(newer)
	c.publish(older)
	if len(delivered) != 1 || delivered[0].revision != newer.revision {
		t.Fatalf("out-of-order publication rolled back the query: %+v", delivered)
	}
	c.Cancel()
	c.publish(newer)
	if len(delivered) != 1 {
		t.Fatal("cancelled query published another snapshot")
	}
}
