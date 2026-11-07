package analytics

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestSessionUUIDv7AndActivityWindows(t *testing.T) {
	set := fake.NewSet()
	c := newTestClient(t, set)
	first, ok := c.TouchSession()
	if !ok {
		t.Fatal("initial session not created")
	}
	firstID := first.SessionID()
	if len(firstID) != 36 || len(strings.ReplaceAll(firstID, "-", "")) != 32 {
		t.Fatalf("UUID = %q", firstID)
	}
	compact := strings.ReplaceAll(firstID, "-", "")
	raw, err := hex.DecodeString(compact)
	if err != nil || raw[6]>>4 != 7 || raw[8]>>6 != 2 {
		t.Fatalf("not UUIDv7: %q", first.id)
	}
	ms := int64(raw[0])<<40 | int64(raw[1])<<32 | int64(raw[2])<<24 | int64(raw[3])<<16 | int64(raw[4])<<8 | int64(raw[5])
	if ms != set.Clock.Now().UnixMilli() {
		t.Fatalf("timestamp = %d, want %d", ms, set.Clock.Now().UnixMilli())
	}
	set.Clock.Advance(sessionIdleLimit - time.Millisecond)
	same, _ := c.TouchSession()
	if same.SessionID() != firstID {
		t.Fatal("session rolled before idle threshold")
	}
	set.Clock.Advance(sessionIdleLimit)
	rolled, _ := c.TouchSession()
	if rolled.SessionID() == firstID || !c.SessionSnapshotValid(first) {
		t.Fatal("idle rollover or prior snapshot validity incorrect")
	}
}

func TestSessionMaximumAgeAndConsentEpoch(t *testing.T) {
	set := fake.NewSet()
	c := newTestClient(t, set)
	old, _ := c.TouchSession()
	for i := 0; i < 95; i++ {
		set.Clock.Advance(15 * time.Minute)
		active, _ := c.TouchSession()
		if active.SessionID() != old.SessionID() {
			t.Fatal("active session rolled before its maximum age")
		}
	}
	set.Clock.Advance(15 * time.Minute)
	newSnapshot, _ := c.TouchSession()
	if newSnapshot.SessionID() == old.SessionID() || !c.SessionSnapshotValid(old) {
		t.Fatal("max-age rollover semantics incorrect")
	}
	c.SetConsent(false)
	if c.SessionSnapshotValid(newSnapshot) {
		t.Fatal("consent off did not invalidate snapshot")
	}
	c.SetConsent(true)
	if c.SessionSnapshotValid(newSnapshot) {
		t.Fatal("consent on revived snapshot")
	}
	current, ok := c.TouchSession()
	if !ok || current.SessionID() == newSnapshot.SessionID() {
		t.Fatal("consent on did not wait for fresh activity")
	}
}

func TestSessionConcurrentTouchesShareID(t *testing.T) {
	set := fake.NewSet()
	c := newTestClient(t, set)
	const count = 64
	start := make(chan struct{})
	results := make(chan SessionSnapshot, count)
	for i := 0; i < count; i++ {
		go func() {
			<-start
			snapshot, ok := c.TouchSession()
			if !ok {
				t.Error("touch rejected")
			}
			results <- snapshot
		}()
	}
	close(start)
	want := ""
	for i := 0; i < count; i++ {
		got := (<-results).SessionID()
		if want == "" {
			want = got
		} else if got != want {
			t.Fatalf("concurrent touches returned %q and %q", want, got)
		}
	}
}

func TestSessionConcurrentConsentCyclesInvalidateOldSnapshots(t *testing.T) {
	set := fake.NewSet()
	c := newTestClient(t, set)
	old, _ := c.TouchSession()
	const count = 64
	start := make(chan struct{})
	done := make(chan struct{}, count)
	for i := 0; i < count; i++ {
		go func() { <-start; c.SetConsent(false); c.SetConsent(true); _, _ = c.TouchSession(); done <- struct{}{} }()
	}
	close(start)
	for i := 0; i < count; i++ {
		<-done
	}
	if c.SessionSnapshotValid(old) {
		t.Fatal("consent cycle kept old snapshot valid")
	}
	fresh, ok := c.TouchSession()
	if !ok || fresh.SessionID() == old.SessionID() {
		t.Fatal("final consent state did not create fresh activity")
	}
	if c.SessionSnapshotValid(fresh) != true {
		t.Fatal("fresh snapshot invalid")
	}
}

func TestDisabledOrNilClientDoesNotCreateSession(t *testing.T) {
	var nilClient *Client
	if _, ok := nilClient.TouchSession(); ok || nilClient.SessionSnapshotValid(SessionSnapshot{}) {
		t.Fatal("nil client accepted session")
	}
	set := fake.NewSet()
	c := New(Config{Clock: set.Clock, Consent: true})
	if _, ok := c.TouchSession(); ok {
		t.Fatal("disabled reporting minted session")
	}
	set2 := fake.NewSet()
	local := New(Config{Clock: set2.Clock, Consent: true, Endpoint: IngestEndpoint})
	local.cfg.Endpoint = ""
	if _, ok := local.TouchSession(); ok {
		t.Fatal("local-only reporting minted session")
	}
}
