package analytics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestObserveInstallOwnerHashesAndDeduplicates(t *testing.T) {
	set := fake.NewSet()
	set.HTTP.Script(fake.Response{Status: http.StatusOK})
	c := newTestClient(t, set)
	if !c.ObserveInstallOwner("private-account") || c.ObserveInstallOwner("private-account") {
		t.Fatal("owner observation should be accepted once")
	}
	if !c.ObserveInstallOwner("another-account") {
		t.Fatal("different owner should be accepted")
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	events := sentBatches(t, set)[0].Batch
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	wantHash := sha256.Sum256([]byte("private-account"))
	if events[0].Properties["owner_id"] != hex.EncodeToString(wantHash[:]) || events[0].Properties["owner_source"] != "account" {
		t.Fatalf("owner properties = %v", events[0].Properties)
	}
	if strings.Contains(string(set.HTTP.Requests()[0].Body), "private-account") || events[0].DistinctID == "" || events[0].DistinctID == events[0].Properties["owner_id"] {
		t.Fatal("account ID leaked or anonymous distinct ID changed")
	}
	if events[0].Properties["$process_person_profile"] != false {
		t.Fatal("person profile enabled")
	}
}

func TestObserveInstallOwnerConsentAndQueueRejectionDoNotMark(t *testing.T) {
	set := fake.NewSet()
	c := newTestClient(t, set)
	c.SetConsent(false)
	if c.ObserveInstallOwner("id") {
		t.Fatal("accepted with consent off")
	}
	c.SetConsent(true)
	if !c.ObserveInstallOwner("id") {
		t.Fatal("did not accept after consent enabled")
	}
	c = newTestClient(t, fake.NewSet())
	for i := 0; i < maxQueue; i++ {
		c.AppLaunched()
	}
	if c.ObserveInstallOwner("id") {
		t.Fatal("accepted into full queue")
	}
	c.mu.Lock()
	c.queue = nil
	c.mu.Unlock()
	if !c.ObserveInstallOwner("id") {
		t.Fatal("queue rejection incorrectly marked owner")
	}
}

func TestObserveInstallOwnerAcceptedThenOptOutAndBackInRemainsDeduplicated(t *testing.T) {
	c := newTestClient(t, fake.NewSet())
	if !c.ObserveInstallOwner("id") {
		t.Fatal("owner was not accepted")
	}
	c.SetConsent(false)
	if c.Pending() != 0 {
		t.Fatalf("pending after opt out = %d", c.Pending())
	}
	c.SetConsent(true)
	if c.ObserveInstallOwner("id") {
		t.Fatal("accepted owner again after consent returned")
	}
}

func TestConcurrentOwnerObservationQueuesOnce(t *testing.T) {
	c := newTestClient(t, fake.NewSet())
	var wg sync.WaitGroup
	var accepted atomic.Int32
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if c.ObserveInstallOwner("id") {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := accepted.Load(); got != 1 {
		t.Fatalf("accepted observations = %d, want 1", got)
	}
	if got := c.Pending(); got != 1 {
		t.Fatalf("pending = %d, want 1", got)
	}
}

func TestObserveInstallOwnerDoesNotWaitForFlushNetwork(t *testing.T) {
	set := fake.NewSet()
	entered, release := make(chan struct{}), make(chan struct{})
	var enteredOnce sync.Once
	set.HTTP.Handle(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		enteredOnce.Do(func() { close(entered) })
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	c := newTestClient(t, set)
	c.AppLaunched()
	flushDone := make(chan struct{})
	go func() { _ = c.Flush(context.Background()); close(flushDone) }()
	<-entered
	ownerDone := make(chan bool, 1)
	go func() { ownerDone <- c.ObserveInstallOwner("id") }()
	select {
	case accepted := <-ownerDone:
		if !accepted {
			t.Fatal("owner was rejected")
		}
	case <-time.After(time.Second):
		t.Fatal("owner observation blocked on network flush")
	}
	close(release)
	<-flushDone
}
