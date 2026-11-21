package analytics

import (
	"context"
	"net/http"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestViewActionCaptureAndFeatureDedupe(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: http.StatusOK})
	c := newTestClient(t, set)
	session, _ := c.TouchSession()
	completion := Completion{Outcome: OutcomeSuccess, BootID: "0123456789abcdef0123456789abcdef", JourneyID: "abcdef0123456789abcdef0123456789", ActionID: "fedcba9876543210fedcba9876543210", Session: session}
	if !c.ViewActionCompleted(completion) || !c.ViewActionUsed(completion, "extension-action:sample") || c.ViewActionUsed(completion, "") || !c.CommandUsed(completion, "settings") {
		t.Fatal("unexpected capture or dedupe")
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	events := sentBatches(t, set)[0].Batch
	if len(events) != 3 {
		t.Fatalf("events = %d", len(events))
	}
	op := events[0].Properties
	if op["operation"] != "view.action" || op["label"] != "Extension view action" || op["stage"] != "invoke" || op["$session_id"] != session.SessionID() {
		t.Fatalf("operation = %+v", op)
	}
	usage := events[1].Properties
	if usage["feature"] != FeatureViewAction || usage["variant"] != "extension-action:sample" || usage["$session_id"] != session.SessionID() {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestViewActionEligibilityAndVariants(t *testing.T) {
	c := newTestClient(t, fake.NewSet())
	completion := Completion{Outcome: OutcomeCancelled, BootID: "0123456789abcdef0123456789abcdef", JourneyID: "abcdef0123456789abcdef0123456789", ActionID: "fedcba9876543210fedcba9876543210"}
	if c.ViewActionUsed(completion, "") {
		t.Fatal("cancelled")
	}
	completion.Outcome = OutcomeSuccess
	c.SetConsent(false)
	if c.ViewActionUsed(completion, "") {
		t.Fatal("consent off")
	}
	c.SetConsent(true)
	for _, bad := range []string{"settings", "extension-action:", "extension-action:private text", "extension-command:sample", "user-extension-secret"} {
		if c.ViewActionUsed(completion, bad) {
			t.Fatalf("accepted %q", bad)
		}
	}
	variant, ok := BuiltInViewActionVariant("sample")
	if !ok || variant != "extension-action:sample" || !c.ViewActionUsed(completion, variant) {
		t.Fatal("valid variant rejected")
	}
	if _, ok := BuiltInViewActionVariant("private text"); ok {
		t.Fatal("invalid ID accepted")
	}
}
