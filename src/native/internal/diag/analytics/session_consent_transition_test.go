package analytics

import (
	"runtime"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestSetConsentOffHoldsSessionLockThroughQueueDiscard(t *testing.T) {
	c := newTestClient(t, fake.NewSet())
	if !c.AppLaunched() {
		t.Fatal("initial event was not accepted")
	}

	// Block the real discard at the queue mutex, then observe the real consent transition.
	c.mu.Lock()
	offDone := make(chan struct{})
	go func() {
		c.SetConsent(false)
		close(offDone)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for c.Consent() && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	consentChanged := !c.Consent()
	transitionLockHeld := !c.sessionMu.TryLock()
	if !transitionLockHeld {
		c.sessionMu.Unlock()
	}

	// Unblock and join before failing, so a regression cannot strand the test goroutine.
	c.mu.Unlock()
	select {
	case <-offDone:
	case <-time.After(5 * time.Second):
		t.Fatal("opt-out did not finish after releasing queue mutex")
	}
	if !consentChanged {
		t.Fatal("opt-out did not reach the consent store before the watchdog")
	}
	if !transitionLockHeld {
		t.Fatal("opt-out released sessionMu before its queue discard completed")
	}

	c.SetConsent(true)
	if !c.AppLaunched() {
		t.Fatal("event was not accepted after opt-in")
	}
	if got := c.Pending(); got != 1 {
		t.Fatalf("pending events = %d, want newly accepted event only", got)
	}
}
