package manager

import (
	"testing"
	"time"
)

// scheduleTitle is the title of the one row the schedule fixture answers the query with.
func scheduleTitle(t *testing.T, h *harness, query string) string {
	t.Helper()
	result := resultFor(h.search(query), "acme.schedule")
	if result == nil || len(result.Items) != 1 {
		t.Fatalf("%q: the fixture did not answer with one row", query)
	}
	return result.Items[0].Title
}

// TestScheduledRunFiresThroughTheExtensionAndCancelDropsIt runs the one-shot trigger through a child process: a due run
// calls the extension's own action, and a cancelled run never does.
func TestScheduledRunFiresThroughTheExtensionAndCancelDropsIt(t *testing.T) {
	h := newHarness(t)
	if err := h.m.Install("acme.schedule", fixtureContents(t, "acme.schedule"), true); err != nil {
		t.Fatalf("Install: %v", err)
	}
	h.waitRunning("acme.schedule")

	h.search("arm 10")
	if got := scheduleTitle(t, h, "pending"); got != "true" {
		t.Fatalf("pending after arm = %q, want true", got)
	}
	deadline := time.Now().Add(waitTimeout)
	for scheduleTitle(t, h, "fired") != "1" {
		if time.Now().After(deadline) {
			t.Fatal("the due run never called the extension's action")
		}
		time.Sleep(5 * time.Millisecond)
	}

	h.search("arm 3600000")
	if got := scheduleTitle(t, h, "cancel"); got != "true" {
		t.Fatalf("cancel of a pending run = %q, want true", got)
	}
	if got := scheduleTitle(t, h, "pending"); got != "false" {
		t.Fatalf("pending after cancel = %q, want false", got)
	}
	if got := scheduleTitle(t, h, "fired"); got != "1" {
		t.Fatalf("fired after cancel = %q, want 1", got)
	}
}
