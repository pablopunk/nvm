package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestCommandCompletedSessionAttributionWireContract(t *testing.T) {
	set := fake.NewSet()
	client := newTestClient(t, set)
	client.cfg.Release = "release-session"

	first, ok := client.TouchSession()
	if !ok || first.SessionID() == "" {
		t.Fatal("TouchSession did not create a session")
	}
	if len(first.SessionID()) != 36 || first.SessionID()[14] != '7' {
		t.Fatalf("session ID is not UUIDv7: %q", first.SessionID())
	}

	setAndFlush := func(completion Completion) map[string]any {
		t.Helper()
		previousBatches := len(sentBatches(t, set))
		set.HTTP.Script(fake.Response{Status: 200})
		if !client.CommandCompleted(completion) {
			t.Fatal("valid command completion was not queued")
		}
		if err := client.Flush(context.Background()); err != nil {
			t.Fatalf("Flush: %v", err)
		}
		batches := sentBatches(t, set)
		if len(batches) != previousBatches+1 || len(batches[previousBatches].Batch) != 1 {
			t.Fatalf("unexpected batches: %#v", batches)
		}
		return batches[previousBatches].Batch[0].Properties
	}
	completion := Completion{Outcome: OutcomeSuccess, Duration: 9 * time.Millisecond,
		BootID: "0123456789abcdef0123456789abcdef", JourneyID: "abcdef0123456789abcdef0123456789",
		ActionID: "fedcba9876543210fedcba9876543210", Session: first}
	props := setAndFlush(completion)
	if props["$session_id"] != first.SessionID() || props["release"] != "release-session" {
		t.Fatalf("session or release missing: %#v", props)
	}
	for _, key := range []string{"operation", "label", "outcome", "stage", "duration_ms", "journey_id", "action_id"} {
		if _, ok := props[key]; !ok {
			t.Errorf("common property %q missing: %#v", key, props)
		}
	}
	for _, key := range []string{"prompt", "input", "error", "private_context"} {
		if _, ok := props[key]; ok {
			t.Errorf("private canary %q included", key)
		}
	}
	withoutSession := completion
	withoutSession.Session = SessionSnapshot{}
	if props := setAndFlush(withoutSession); props["$session_id"] != nil {
		t.Fatalf("absent snapshot was attributed: %#v", props)
	}

	set.Clock.Advance(sessionIdleLimit)
	second, ok := client.TouchSession()
	if !ok || second.SessionID() == first.SessionID() {
		t.Fatal("idle rollover did not create a new session")
	}
	props = setAndFlush(completion)
	if props["$session_id"] != first.SessionID() {
		t.Fatalf("long operation attribution = %#v, want start session", props["$session_id"])
	}

	client.SetConsent(false)
	if client.CommandCompleted(completion) {
		t.Fatal("consent-off accepted completion")
	}
	client.SetConsent(true)
	fresh, ok := client.TouchSession()
	if !ok || fresh.SessionID() == second.SessionID() {
		t.Fatal("consent re-enable did not create fresh session")
	}
	props = setAndFlush(completion)
	if _, ok := props["$session_id"]; ok {
		t.Fatalf("invalidated old snapshot attributed: %#v", props)
	}

	failedJob := JobCompletion{Outcome: OutcomeFailed, Duration: time.Millisecond,
		BootID: completion.BootID, JourneyID: completion.JourneyID, ActionID: completion.ActionID, Session: fresh}
	set.Clock.Advance(29 * time.Minute)
	set.HTTP.Script(fake.Response{Status: 200})
	if !client.JobCompleted(failedJob) {
		t.Fatal("failed job was suppressed")
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatalf("Flush job: %v", err)
	}
	batches := sentBatches(t, set)
	job := batches[len(batches)-1].Batch[0].Properties
	if _, ok := job["$session_id"]; ok {
		t.Fatalf("job had session: %#v", job)
	}
	if job["operation"] != "job.run" {
		t.Fatalf("expected job completion, got %#v", job)
	}
	set.Clock.Advance(time.Minute)
	if next, ok := client.TouchSession(); !ok || next.SessionID() == fresh.SessionID() {
		t.Fatal("job completion kept the person session alive")
	}
	if client.JobCompleted(JobCompletion{Outcome: OutcomeSuccess, BootID: completion.BootID, JourneyID: completion.JourneyID, ActionID: completion.ActionID, Session: fresh}) {
		t.Fatal("successful job was retained")
	}
}
