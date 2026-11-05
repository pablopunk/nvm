package timing

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func newTestTracker(t *testing.T) (*fake.Set, *analytics.Client) {
	t.Helper()
	set := fake.NewSet()
	client := analytics.New(analytics.Config{Endpoint: "https://ingest.test/api/ingest/batch/", APIKey: analytics.IngestKey,
		Transport: set.HTTP, FS: set.FS, Clock: set.Clock, Dir: "/config", Version: "0.9.0", Release: "test-release", Consent: true})
	return set, client
}

func payloads(t *testing.T, set *fake.Set) []map[string]any {
	t.Helper()
	var result []map[string]any
	for _, req := range set.HTTP.Requests() {
		var payload struct {
			Batch []struct {
				Event      string         `json:"event"`
				Properties map[string]any `json:"properties"`
			} `json:"batch"`
		}
		if err := json.Unmarshal(req.Body, &payload); err != nil {
			t.Fatal(err)
		}
		for _, event := range payload.Batch {
			if event.Event == analytics.EventOperationCompleted {
				result = append(result, event.Properties)
			}
		}
	}
	return result
}

func TestFinishSerializesCorrelatedCompletionsOnce(t *testing.T) {
	set, client := newTestTracker(t)
	set.HTTP.Script(fake.Response{Status: 200}, fake.Response{Status: 200}, fake.Response{Status: 200})
	tracker := New(set.Clock, client)
	rootCtx, root := tracker.Start(context.Background())
	nestedCtx, nested := tracker.Start(rootCtx)
	_, independent := tracker.Start(context.Background())
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(payloads(t, set)) != 0 {
		t.Fatal("event sent before Finish")
	}
	set.Clock.Advance(1500 * time.Microsecond)
	nested.Finish(analytics.OutcomeSuccess)
	root.Finish(analytics.OutcomeFailed)
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	props := payloads(t, set)
	if len(props) != 2 {
		t.Fatalf("serialized events=%d, want 2", len(props))
	}
	byAction := map[string]map[string]any{}
	for _, p := range props {
		byAction[p["action_id"].(string)] = p
	}
	rootIDs := rootCtx.Value(contextKey{}).(correlation)
	nestedIDs := nestedCtx.Value(contextKey{}).(correlation)
	independentIDs := independent.ids
	if rootIDs.boot != nestedIDs.boot || rootIDs.boot != independentIDs.boot {
		t.Fatal("boot ID changed within tracker")
	}
	if rootIDs.journey != nestedIDs.journey || rootIDs.journey == independentIDs.journey {
		t.Fatal("journey correlation incorrect")
	}
	if rootIDs.action == nestedIDs.action {
		t.Fatal("nested action ID reused")
	}
	for _, pair := range []struct {
		ids     correlation
		outcome string
	}{{rootIDs, "failed"}, {nestedIDs, "success"}} {
		p := byAction[pair.ids.action]
		if p == nil || p["boot_id"] != pair.ids.boot || p["journey_id"] != pair.ids.journey || p["outcome"] != pair.outcome || p["duration_ms"] != float64(2) || p["release"] != "test-release" {
			t.Fatalf("serialized completion mismatch: %#v", p)
		}
		if _, ok := p["context_canary"]; ok {
			t.Fatal("context value leaked")
		}
	}
	for i := 0; i < 8; i++ {
		nested.Finish(analytics.OutcomeFailed)
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(payloads(t, set)); got != 2 {
		t.Fatalf("repeat Finish sent %d events", got)
	}
}

func TestConsentWithdrawalAndCancellation(t *testing.T) {
	set, client := newTestTracker(t)
	tracker := New(set.Clock, client)
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), "canary", "private"))
	ctx, done := tracker.Start(parent)
	cancel()
	if ctx.Err() != context.Canceled {
		t.Fatal("parent cancellation was not retained")
	}
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(payloads(t, set)) != 0 {
		t.Fatal("cancellation auto-reported")
	}
	done.Finish(analytics.OutcomeCancelled)
	client.SetConsent(false)
	client.SetConsent(true)
	done.Finish(analytics.OutcomeSuccess)
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(payloads(t, set)) != 0 {
		t.Fatal("withdrawn completion replayed")
	}
}

func TestNilCompletionAndDeadline(t *testing.T) {
	var absent *Completion
	absent.Finish(analytics.OutcomeUnknown)
	set, client := newTestTracker(t)
	tracker := New(set.Clock, client)
	parent, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Hour))
	defer cancel()
	ctx, completion := tracker.Start(parent)
	deadline, ok := ctx.Deadline()
	if !ok || deadline.IsZero() {
		t.Fatal("deadline was not retained")
	}
	completion.Finish(analytics.OutcomeUnknown)
	noSink := New(set.Clock, nil)
	_, nilSinkDone := noSink.Start(context.Background())
	nilSinkDone.Finish(analytics.OutcomeSuccess)
}
