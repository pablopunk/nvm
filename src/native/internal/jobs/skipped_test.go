package jobs

import (
	"context"
	"testing"
	"time"
)

func TestOperationPrerequisiteSkipDoesNotFailHaltOrConsumeBudget(t *testing.T) {
	h := newHarness(t)
	mustRegister(t, h, Definition{ID: "prerequisite", Title: "Prerequisite", Budget: Budget{MaxRuns: 1, Window: time.Hour}, Run: func(context.Context, Run) error { return Skipped{Reason: "not available"} }})
	for count := 1; count <= 2; count++ {
		if err := h.sched.RunNow("prerequisite"); err != nil {
			t.Fatal(err)
		}
		snapshot := h.awaitJob("prerequisite", func(s Snapshot) bool { return len(s.History) == count && !s.Running })
		if snapshot.History[0].Outcome != OutcomeSkipped || snapshot.History[0].Error != "not available" || snapshot.Failures != 0 {
			t.Fatalf("skip counted as a failure or budget refusal: %+v", snapshot)
		}
	}
	if observedOutcome(Skipped{Reason: "blocked"}) != RunSkipped {
		t.Fatal("skip observer outcome lost")
	}
}
