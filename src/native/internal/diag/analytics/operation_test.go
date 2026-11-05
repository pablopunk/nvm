package analytics

import (
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
		BootID: "0123456789abcdef0123456789abcdef",
		JourneyID: "abcdef0123456789abcdef0123456789",
		ActionID: "fedcba9876543210fedcba9876543210",
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
