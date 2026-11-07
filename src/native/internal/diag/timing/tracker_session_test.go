package timing

import (
	"context"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/diag/analytics"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestPersonOperationKeepsStartSessionAcrossRollover(t *testing.T) {
	set, client := newTestTracker(t)
	set.HTTP.Script(fake.Response{Status: 200}, fake.Response{Status: 200})
	tracker := New(set.Clock, client)
	_, older := tracker.Start(context.Background())
	oldID := older.session.SessionID()
	if oldID == "" {
		t.Fatal("person start did not snapshot a session")
	}
	set.Clock.Advance(30 * time.Minute)
	_, newer := tracker.Start(context.Background())
	newID := newer.session.SessionID()
	if newID == "" || newID == oldID {
		t.Fatal("idle person start did not create a new session")
	}
	older.Finish(analytics.OutcomeSuccess)
	newer.Finish(analytics.OutcomeSuccess)
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	props := payloads(t, set)
	if len(props) != 2 || props[0]["$session_id"] != oldID || props[1]["$session_id"] != newID {
		t.Fatalf("session attribution: %#v", props)
	}
}

func TestConsentEpochDropsOldSessionAndDisabledReportingDoesNotMint(t *testing.T) {
	set, client := newTestTracker(t)
	set.HTTP.Script(fake.Response{Status: 200})
	tracker := New(set.Clock, client)
	_, old := tracker.Start(context.Background())
	if old.session.SessionID() == "" {
		t.Fatal("missing initial session")
	}
	client.SetConsent(false)
	client.SetConsent(true)
	_, current := tracker.Start(context.Background())
	if current.session.SessionID() == "" {
		t.Fatal("consent restoration did not permit a fresh session")
	}
	old.Finish(analytics.OutcomeSuccess)
	if err := client.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := payloads(t, set); len(got) != 1 || got[0]["$session_id"] != nil {
		t.Fatalf("old snapshot was attributed: %#v", got)
	}

	disabled := analytics.New(analytics.Config{Clock: set.Clock, FS: set.FS, Dir: "/config", Consent: true})
	_, noReport := New(set.Clock, disabled).Start(context.Background())
	if noReport.session.SessionID() != "" {
		t.Fatal("disabled reporting minted a session")
	}
}

func TestBackgroundOperationsDoNotTouchPersonSession(t *testing.T) {
	set, client := newTestTracker(t)
	tracker := New(set.Clock, client)
	_, person := tracker.Start(context.Background())
	firstID := person.session.SessionID()
	set.Clock.Advance(29 * time.Minute)
	jobCtx, job := tracker.StartJob(context.Background())
	_, nested := tracker.Start(jobCtx)
	if job.session.SessionID() != "" || nested.session.SessionID() != "" {
		t.Fatal("background operation captured a session")
	}
	set.Clock.Advance(time.Minute)
	_, next := tracker.Start(context.Background())
	if next.session.SessionID() == firstID {
		t.Fatal("background operation advanced person activity")
	}
}

type unsupportedSink struct{}

func (unsupportedSink) CommandCompleted(analytics.CommandCompletion) bool { return true }

func TestSessionSourceIsOptional(t *testing.T) {
	set := fake.NewSet()
	_, completion := New(set.Clock, unsupportedSink{}).Start(context.Background())
	if completion.session.SessionID() != "" {
		t.Fatal("unsupported source produced a session")
	}
	completion.Finish(analytics.OutcomeSuccess)
}
