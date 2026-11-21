package agent

import (
	"testing"
)

// A tool's goroutine can report after the turn's loop ended, as a code mode program's late call does. The report is
// dropped instead of panicking on the closed event channel or blocking a reader that stopped reading.
func TestReportsAfterTheTurnEndedAreDropped(t *testing.T) {
	turn := &Turn{events: make(chan Event, 1)}
	r := &turnRun{turn: turn}
	r.finish(Event{Kind: EventAborted})
	r.emit(Event{Kind: EventToolEnd})
	r.closeEvents()
	r.emit(Event{Kind: EventToolEnd})
	var got []Event
	for event := range turn.events {
		got = append(got, event)
	}
	if len(got) != 1 || got[0].Kind != EventAborted {
		t.Fatalf("events = %+v, want only the terminal event", got)
	}
}
