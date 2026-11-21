package analytics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestCommandUsedPreservesStartSessionWithoutTouchingActivity(t *testing.T) {
	set := fake.NewSet()
	c := newTestClient(t, set)
	c.cfg.Release = "release-usage"
	start, _ := c.TouchSession()
	completion := Completion{Outcome: OutcomeSuccess, BootID: "0123456789abcdef0123456789abcdef", ActionID: "fedcba9876543210fedcba9876543210", Session: start}
	capture := func(journey int) map[string]any {
		t.Helper()
		completion.JourneyID = fmt.Sprintf("%032x", journey)
		set.HTTP.Script(fake.Response{Status: 200})
		if !c.CommandUsed(completion, "user-extension") {
			t.Fatal("usage not accepted")
		}
		if err := c.Flush(context.Background()); err != nil {
			t.Fatal(err)
		}
		batches := sentBatches(t, set)
		return batches[len(batches)-1].Batch[0].Properties
	}
	set.Clock.Advance(sessionIdleLimit)
	rolled, _ := c.TouchSession()
	if rolled.SessionID() == start.SessionID() {
		t.Fatal("session did not roll over")
	}
	set.Clock.Advance(29 * time.Minute)
	props := capture(1)
	if props["$session_id"] != start.SessionID() || props["release"] != "release-usage" {
		t.Fatalf("start attribution lost: %#v", props)
	}
	set.Clock.Advance(time.Minute)
	after, _ := c.TouchSession()
	if after.SessionID() == rolled.SessionID() {
		t.Fatal("completion touched activity")
	}
	c.SetConsent(false)
	c.SetConsent(true)
	props = capture(2)
	if _, exists := props["$session_id"]; exists {
		t.Fatal("ended session was restored")
	}
	completion.Session = SessionSnapshot{}
	props = capture(3)
	if _, exists := props["$session_id"]; exists {
		t.Fatal("absent session was invented")
	}
}

func TestCommandUsedRejectsInvalidContextWithoutConsumingDedupe(t *testing.T) {
	valid := Completion{Outcome: OutcomeSuccess, BootID: "0123456789abcdef0123456789abcdef", JourneyID: "abcdef0123456789abcdef0123456789", ActionID: "fedcba9876543210fedcba9876543210"}
	c := newTestClient(t, fake.NewSet())
	for _, field := range []string{"boot", "journey", "action", "outcome"} {
		completion := valid
		switch field {
		case "boot":
			completion.BootID = "private boot"
		case "journey":
			completion.JourneyID = "private journey"
		case "action":
			completion.ActionID = "private action"
		case "outcome":
			completion.Outcome = "private outcome"
		}
		if c.CommandUsed(completion, "") {
			t.Fatalf("invalid %s accepted", field)
		}
	}
	if !c.CommandUsed(valid, "") {
		t.Fatal("invalid attempts consumed dedupe")
	}
	var absent *Client
	if absent.CommandUsed(valid, "") {
		t.Fatal("nil client accepted use")
	}
	development := New(Config{Consent: true})
	if development.CommandUsed(valid, "") || len(development.commandUsage.keys) != 0 {
		t.Fatal("development client recorded usage")
	}
}
