package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestCommandCompletedQueuesClosedSchema(t *testing.T) {
	set := fake.NewSet()
	client := newTestClient(t, set)
	client.cfg.Release = "release-1"
	completion := CommandCompletion{
		Outcome: OutcomeSuccess, Duration: 1499 * time.Microsecond,
		BootID:    "0123456789abcdef0123456789abcdef",
		JourneyID: "abcdef0123456789abcdef0123456789",
		ActionID:  "fedcba9876543210fedcba9876543210",
	}
	if !client.CommandCompleted(completion) {
		t.Fatal("valid completion was not queued")
	}
	if client.Pending() != 1 {
		t.Fatalf("pending = %d, want 1", client.Pending())
	}
	event := client.queue[0]
	if event.event != EventOperationCompleted {
		t.Fatalf("event = %q", event.event)
	}
	want := map[string]any{
		"operation": "command.execute", "label": "Extension command", "outcome": "success", "stage": "invoke",
		"duration_ms": int64(1), "boot_id": completion.BootID, "journey_id": completion.JourneyID,
		"action_id": completion.ActionID, "release": "release-1",
	}
	for key, value := range want {
		if event.props[key] != value {
			t.Errorf("%s = %#v, want %#v", key, event.props[key], value)
		}
	}
	if len(event.props) != len(want)+len(client.common) {
		t.Fatalf("unexpected event properties: %#v", event.props)
	}
	for _, key := range []string{"command_id", "title", "prompt", "clipboard", "user_id", "email"} {
		if _, exists := event.props[key]; exists {
			t.Errorf("private or dynamic field %q present", key)
		}
	}
}

func TestJobCompletedWireContractAndSuccessFilter(t *testing.T) {
	for _, outcome := range []OperationOutcome{OutcomeFailed, OutcomeCancelled, OutcomeTimedOut} {
		t.Run(string(outcome), func(t *testing.T) {
			set := fake.NewSet()
			set.HTTP.Script(fake.Response{Status: 200})
			client := newTestClient(t, set)
			client.cfg.Release = "release-2"
			completion := JobCompletion{Outcome: outcome, Duration: 12 * time.Millisecond,
				BootID: "0123456789abcdef0123456789abcdef", JourneyID: "abcdef0123456789abcdef0123456789",
				ActionID: "fedcba9876543210fedcba9876543210"}
			if !client.JobCompleted(completion) {
				t.Fatal("job failure was not queued")
			}
			if err := client.Flush(context.Background()); err != nil {
				t.Fatalf("Flush: %v", err)
			}
			batches := sentBatches(t, set)
			if len(batches) != 1 || len(batches[0].Batch) != 1 {
				t.Fatalf("batches = %#v", batches)
			}
			props := batches[0].Batch[0].Properties
			for key, want := range map[string]any{"operation": "job.run", "outcome": string(outcome), "stage": "dispatch", "duration_ms": float64(12), "boot_id": completion.BootID, "journey_id": completion.JourneyID, "action_id": completion.ActionID, "release": "release-2"} {
				if props[key] != want {
					t.Errorf("%s = %#v, want %#v", key, props[key], want)
				}
			}
			if _, ok := props["label"]; ok {
				t.Error("job label must be absent")
			}
			for _, key := range []string{"job_id", "title", "payload", "error", "prompt", "clipboard"} {
				if _, ok := props[key]; ok {
					t.Errorf("private field %q present", key)
				}
			}
		})
	}
	set := fake.NewSet()
	client := newTestClient(t, set)
	completion := JobCompletion{Outcome: OutcomeSuccess, BootID: "0123456789abcdef0123456789abcdef", JourneyID: "0123456789abcdef0123456789abcdef", ActionID: "0123456789abcdef0123456789abcdef"}
	if client.JobCompleted(completion) || client.Pending() != 0 {
		t.Fatal("routine job success was retained")
	}
	if !client.CommandCompleted(CommandCompletion(completion)) {
		t.Fatal("command success was suppressed")
	}
}

func TestJobCompletedRejectsInvalidAndHonorsConsent(t *testing.T) {
	var nilClient *Client
	valid := JobCompletion{Outcome: OutcomeFailed, BootID: "0123456789abcdef0123456789abcdef", JourneyID: "0123456789abcdef0123456789abcdef", ActionID: "0123456789abcdef0123456789abcdef"}
	if nilClient.JobCompleted(valid) {
		t.Fatal("nil client accepted job")
	}
	client := newTestClient(t, fake.NewSet())
	invalid := valid
	invalid.Outcome = "custom"
	if client.JobCompleted(invalid) {
		t.Fatal("invalid outcome accepted")
	}
	invalid = valid
	invalid.ActionID = "bad"
	if client.JobCompleted(invalid) {
		t.Fatal("invalid id accepted")
	}
	if !client.JobCompleted(valid) {
		t.Fatal("valid job not queued")
	}
	client.SetConsent(false)
	if client.Pending() != 0 || client.JobCompleted(valid) {
		t.Fatal("withdrawn consent retained or accepted job")
	}
}

func TestCommandCompletedValidationConsentAndNil(t *testing.T) {
	var nilClient *Client
	valid := CommandCompletion{Outcome: OutcomeFailed, Duration: -time.Second,
		BootID: "0123456789abcdef0123456789abcdef", JourneyID: "0123456789abcdef0123456789abcdef",
		ActionID: "0123456789abcdef0123456789abcdef"}
	if nilClient.CommandCompleted(valid) {
		t.Fatal("nil client accepted event")
	}
	set := fake.NewSet()
	client := newTestClient(t, set)
	invalid := valid
	invalid.Outcome = "maybe"
	if client.CommandCompleted(invalid) {
		t.Fatal("invalid outcome accepted")
	}
	invalid = valid
	invalid.JourneyID = "uuid-not-32-hex"
	if client.CommandCompleted(invalid) {
		t.Fatal("invalid identity accepted")
	}
	if !client.CommandCompleted(valid) || client.queue[0].props["duration_ms"] != int64(0) {
		t.Fatal("negative duration was not clamped to zero")
	}
	client.SetConsent(false)
	if client.Pending() != 0 || client.CommandCompleted(valid) {
		t.Fatal("withdrawn consent retained or accepted event")
	}
}

func TestCommandCompletedOmitsUnavailableRelease(t *testing.T) {
	set := fake.NewSet()
	client := newTestClient(t, set)
	client.cfg.Release = ""
	completion := CommandCompletion{Outcome: OutcomeUnknown, BootID: "0123456789abcdef0123456789abcdef",
		JourneyID: "0123456789abcdef0123456789abcdef", ActionID: "0123456789abcdef0123456789abcdef"}
	if !client.CommandCompleted(completion) {
		t.Fatal("valid completion was not queued")
	}
	if _, exists := client.queue[0].props["release"]; exists {
		t.Fatal("unavailable release was included")
	}
}
