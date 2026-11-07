package analytics

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestCommandUsedCapturesCanonicalPropertiesAndDedupes(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: http.StatusOK})
	c := newTestClient(t, set)
	session, ok := c.TouchSession()
	if !ok {
		t.Fatal("TouchSession failed")
	}
	completion := Completion{Outcome: OutcomeSuccess, BootID: "0123456789abcdef0123456789abcdef", JourneyID: "abcdef0123456789abcdef0123456789", ActionID: "fedcba9876543210fedcba9876543210", Session: session}
	if !c.CommandUsed(completion, "settings") || c.CommandUsed(completion, "settings") {
		t.Fatal("first use should record once")
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	event := sentBatches(t, set)[0].Batch[0]
	if event.Event != EventFeatureUsed || event.Properties["feature"] != FeatureCommandRun || event.Properties["variant"] != "settings" || event.Properties["outcome"] != "success" || event.Properties["$session_id"] != session.SessionID() {
		t.Fatalf("event = %+v", event)
	}
	if _, exists := event.Properties["release"]; exists {
		t.Fatal("empty release was not omitted")
	}

	for _, key := range []string{"journey_id", "action_id", "boot_id"} {
		if _, exists := event.Properties[key]; exists {
			t.Fatalf("leaked %s", key)
		}
	}
}

func TestCommandUsedCancellationEligibilityAndQueueRejection(t *testing.T) {
	set := fake.NewSet()
	c := newTestClient(t, set)
	completion := Completion{Outcome: OutcomeCancelled, BootID: "0123456789abcdef0123456789abcdef", JourneyID: "abcdef0123456789abcdef0123456789", ActionID: "fedcba9876543210fedcba9876543210"}
	if c.CommandUsed(completion, "") {
		t.Fatal("cancelled command counted")
	}
	completion.Outcome = OutcomeFailed
	c.SetConsent(false)
	if c.CommandUsed(completion, "") {
		t.Fatal("consent-off command counted")
	}
	c.SetConsent(true)
	for range maxQueue {
		if !c.AppLaunched() {
			t.Fatal("could not fill queue")
		}
	}
	if c.CommandUsed(completion, "") {
		t.Fatal("full queue accepted use")
	}
	c.discard()
	if c.CommandUsed(completion, "") {
		t.Fatal("rejected queue use did not consume dedupe key")
	}
	completion.JourneyID = "11111111111111111111111111111111"
	if !c.CommandUsed(completion, "") {
		t.Fatal("eligible failure in a new journey not accepted")
	}
	c.SetConsent(false)
	c.SetConsent(true)
	if c.CommandUsed(completion, "") {
		t.Fatal("consent transition reset journey dedupe")
	}

}

func TestCommandUsedConcurrentAndVariantValidation(t *testing.T) {
	c := newTestClient(t, fake.NewSet())
	completion := Completion{Outcome: OutcomeUnknown, BootID: "0123456789abcdef0123456789abcdef", JourneyID: "abcdef0123456789abcdef0123456789", ActionID: "fedcba9876543210fedcba9876543210"}
	var wg sync.WaitGroup
	var accepted int
	var mu sync.Mutex
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if c.CommandUsed(completion, "extension-command:sample:run") {
				mu.Lock()
				accepted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if accepted != 1 {
		t.Fatalf("accepted = %d", accepted)
	}
	for _, bad := range []string{"https://private", "extension-command:bad id:run", "query=private", "user-extension-secret"} {
		if c.CommandUsed(Completion{Outcome: OutcomeSuccess, BootID: completion.BootID, JourneyID: "11111111111111111111111111111111", ActionID: completion.ActionID}, bad) {
			t.Fatalf("accepted invalid variant %q", bad)
		}
	}
}

func TestCommandUsedDedupeEvictsInInsertionOrder(t *testing.T) {
	c := newTestClient(t, fake.NewSet())
	completion := Completion{Outcome: OutcomeSuccess, BootID: "0123456789abcdef0123456789abcdef", ActionID: "fedcba9876543210fedcba9876543210"}
	for journey := 1; journey <= commandUsageDedupeLimit; journey++ {
		completion.JourneyID = fmt.Sprintf("%032x", journey)
		if !c.CommandUsed(completion, "") {
			t.Fatalf("journey %d not accepted", journey)
		}
		c.discard()
	}
	completion.JourneyID = fmt.Sprintf("%032x", 1)
	if c.CommandUsed(completion, "") {
		t.Fatal("duplicate accepted")
	}
	completion.JourneyID = fmt.Sprintf("%032x", commandUsageDedupeLimit+1)
	if !c.CommandUsed(completion, "") {
		t.Fatal("new journey not accepted")
	}
	completion.JourneyID = fmt.Sprintf("%032x", 2)
	if c.CommandUsed(completion, "") {
		t.Fatal("second-oldest key evicted early")
	}
	completion.JourneyID = fmt.Sprintf("%032x", 1)
	if !c.CommandUsed(completion, "") {
		t.Fatal("oldest key was not evicted")
	}
	if len(c.commandUsage.keys) != commandUsageDedupeLimit || len(c.commandUsage.order) != commandUsageDedupeLimit {
		t.Fatal("dedupe exceeded cap")
	}
}

func TestCommandVariantMatchesSourceIDContract(t *testing.T) {
	for _, ids := range [][2]string{{"Sample", ""}, {"Sample:Part", "Run:Now"}, {strings.Repeat("a", 64), strings.Repeat("b", 64)}} {
		variant, ok := BuiltInCommandVariant("extension-command", ids[0], ids[1])
		if !ok || !validCommandVariant(variant) {
			t.Fatalf("constructor and validator disagree for %v", ids)
		}
	}
	for _, ids := range [][2]string{{"private extension", "run"}, {"valid", "private command"}, {strings.Repeat("a", 65), "run"}} {
		if _, ok := BuiltInCommandVariant("extension-command", ids[0], ids[1]); ok {
			t.Fatalf("accepted invalid source IDs %v", ids)
		}
	}
	if validCommandVariant("extension-command:" + strings.Repeat("a", 130)) {
		t.Fatal("accepted overlong source IDs")
	}
}
