package timing

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/ext/manager"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestCommandUsageEligibilityAndBackgroundInheritance(t *testing.T) {
	for _, tc := range []struct {
		name        string
		marked, job bool
		outcome     manager.RunOutcome
		want        int
	}{
		{"person success", true, false, manager.RunOutcomeSuccess, 1},
		{"person failure", true, false, manager.RunOutcomeFailed, 1},
		{"person timeout", true, false, manager.RunOutcomeTimedOut, 1},
		{"cancelled", true, false, manager.RunOutcomeCancelled, 0},
		{"unmarked", false, false, manager.RunOutcomeSuccess, 0},
		{"job nested success", true, true, manager.RunOutcomeSuccess, 0},
		{"job nested failure", true, true, manager.RunOutcomeFailed, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set, client := newTestTracker(t)
			set.HTTP.Script(fake.Response{Status: 200})
			tracker := New(set.Clock, client)
			ctx := context.Background()
			if tc.job {
				ctx, _ = tracker.StartJob(ctx)
			}
			if tc.marked {
				ctx = manager.WithCommandRunOrigin(ctx, manager.CommandOrigin{})
			}
			observed, finish := ExtensionRunObserver(tracker).BeginRun(ctx)
			if _, _, marked := manager.ConsumeCommandOrigin(observed); marked {
				t.Fatal("observer did not consume marker")
			}
			if finish != nil {
				finish(tc.outcome)
				finish(tc.outcome)
			}
			if err := client.Flush(context.Background()); err != nil {
				t.Fatal(err)
			}
			usage := usageProperties(t, set)
			if len(usage) != tc.want {
				t.Fatalf("usage = %#v, want %d", usage, tc.want)
			}
			if tc.want != 0 && (usage[0]["variant"] != "user-extension" || usage[0]["outcome"] != string(tc.outcome)) {
				t.Fatalf("usage properties = %#v", usage)
			}
			if tc.job {
				for _, props := range payloads(t, set) {
					if props["$session_id"] != nil {
						t.Fatal("job-nested timing acquired a person session")
					}
				}
			}
		})
	}
}

func TestCommandUsageNestedJourneyDedupesAndIndependentJourneyCounts(t *testing.T) {
	set, client := newTestTracker(t)
	set.HTTP.Script(fake.Response{Status: 200})
	tracker := New(set.Clock, client)
	observer := ExtensionRunObserver(tracker)
	ctx, first := observer.BeginRun(manager.WithCommandRun(context.Background()))
	_, nested := observer.BeginRun(manager.WithCommandRun(ctx))
	_, independent := observer.BeginRun(manager.WithCommandRun(context.Background()))
	nested(manager.RunOutcomeSuccess)
	first(manager.RunOutcomeFailed)
	independent(manager.RunOutcomeSuccess)
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := usageProperties(t, set); len(got) != 2 {
		t.Fatalf("usage = %#v, want one per journey", got)
	}
	operations := payloads(t, set)
	if len(operations) != 3 || operations[0]["journey_id"] != operations[1]["journey_id"] || operations[1]["journey_id"] == operations[2]["journey_id"] {
		t.Fatalf("operation journeys = %#v", operations)
	}
}

func TestCommandUsageFinishDoesNotReviveSessionAndUnsupportedSinkIsSafe(t *testing.T) {
	set, client := newTestTracker(t)
	set.HTTP.Script(fake.Response{Status: 200})
	tracker := New(set.Clock, client)
	_, complete := tracker.StartCommand(context.Background(), manager.CommandOrigin{})
	startID := complete.session.SessionID()
	set.Clock.Advance(30 * time.Minute)
	current, _ := client.TouchSession()
	complete.Finish(analytics.OutcomeSuccess)
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if usage := usageProperties(t, set); len(usage) != 1 || usage[0]["$session_id"] != startID || startID == current.SessionID() {
		t.Fatalf("start session not retained: %#v", usage)
	}
	_, absent := New(set.Clock, unsupportedSink{}).StartCommand(context.Background(), manager.CommandOrigin{})
	absent.Finish(analytics.OutcomeSuccess)
}

func usageProperties(t *testing.T, set *fake.Set) []map[string]any {
	t.Helper()
	var props []map[string]any
	for _, event := range commandEvents(t, set) {
		if event["event"] == analytics.EventFeatureUsed {
			props = append(props, event["properties"].(map[string]any))
		}
	}
	return props
}
