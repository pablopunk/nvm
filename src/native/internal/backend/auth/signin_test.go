package auth

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
	"github.com/pablopunk/nvm/native/internal/store"
)

// TestSignInApprovalConnectsTheAccountAndStoresTheToken walks the device flow to approval: the code and page are
// shown, the browser opens the page, the poll waits for approval, and the token lands in the keychain.
func TestSignInApprovalConnectsTheAccountAndStoresTheToken(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.set.HTTP.Script(jsonResponse(200, r.initiateBody(10*time.Minute)), jsonResponse(200, pendingBody), jsonResponse(200, approvedBody))
	if err := r.session.StartSignIn(); err != nil {
		t.Fatalf("StartSignIn: %v", err)
	}
	pending := r.waitForPhase(PhasePending)
	if pending.SignIn.Code != "CODE-123" || !strings.HasPrefix(pending.SignIn.VerificationURL, "https://www.nvm.fyi/") {
		t.Fatalf("pending sign-in = %+v, want the code and the verification page", pending.SignIn)
	}
	if pages := r.waitForBrowsed(1); len(pages) != 1 || pages[0] != pending.SignIn.VerificationURL {
		t.Fatalf("browser opened %v, want the verification page once", pages)
	}

	r.advancePoll()
	r.advancePoll()
	approved := r.waitForPhase(PhaseApproved)
	if !approved.Connected || approved.Email != "pat@example.test" || approved.Role != "member" || approved.UserID != "user_1" {
		t.Fatalf("approved snapshot = %+v, want the connected account", approved)
	}
	token, ok := r.storedKeychainToken(productionAccount)
	if !ok || token != canaryToken {
		t.Fatalf("keychain token = %q (present %v), want the approved token", token, ok)
	}
}

// TestSignInEndsWithDeniedOrExpiredWhenTheBackendSaysSo maps the two refusals to their phases.
func TestSignInEndsWithDeniedOrExpiredWhenTheBackendSaysSo(t *testing.T) {
	t.Run("denied", func(t *testing.T) {
		r := newRig(t, rigOptions{})
		r.set.HTTP.Script(jsonResponse(200, r.initiateBody(10*time.Minute)), jsonResponse(200, `{"status":"denied"}`))
		if err := r.session.StartSignIn(); err != nil {
			t.Fatalf("StartSignIn: %v", err)
		}
		r.waitForPhase(PhasePending)
		r.advancePoll()
		r.waitForPhase(PhaseDenied)
		if r.session.Snapshot().Connected {
			t.Fatal("a denied sign-in must not connect the account")
		}
	})
	t.Run("expired by the backend", func(t *testing.T) {
		r := newRig(t, rigOptions{})
		r.set.HTTP.Script(jsonResponse(200, r.initiateBody(10*time.Minute)), jsonResponse(410, ""))
		if err := r.session.StartSignIn(); err != nil {
			t.Fatalf("StartSignIn: %v", err)
		}
		r.waitForPhase(PhasePending)
		r.advancePoll()
		r.waitForPhase(PhaseExpired)
	})
	t.Run("expired by the clock", func(t *testing.T) {
		r := newRig(t, rigOptions{})
		r.set.HTTP.Script(jsonResponse(200, r.initiateBody(3*time.Second)), jsonResponse(200, pendingBody))
		if err := r.session.StartSignIn(); err != nil {
			t.Fatalf("StartSignIn: %v", err)
		}
		r.waitForPhase(PhasePending)
		r.advancePoll()
		r.advancePoll()
		r.waitForPhase(PhaseExpired)
	})
}

// TestSignInKeepsPollingThroughNetworkFailures checks that a dropped poll does not end the flow.
func TestSignInKeepsPollingThroughNetworkFailures(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.set.HTTP.Script(
		jsonResponse(200, r.initiateBody(10*time.Minute)),
		fake.Response{Err: errors.New("connection reset by peer")},
		jsonResponse(200, approvedBody),
	)
	if err := r.session.StartSignIn(); err != nil {
		t.Fatalf("StartSignIn: %v", err)
	}
	r.waitForPhase(PhasePending)
	r.advancePoll()
	r.advancePoll()
	r.waitForPhase(PhaseApproved)
}

// TestCancelSignInEndsTheWaitAsCancelled checks that cancelling stops the flow without another poll.
func TestCancelSignInEndsTheWaitAsCancelled(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.set.HTTP.Script(jsonResponse(200, r.initiateBody(10*time.Minute)))
	if err := r.session.StartSignIn(); err != nil {
		t.Fatalf("StartSignIn: %v", err)
	}
	r.waitForPhase(PhasePending)
	r.session.CancelSignIn()
	r.waitForPhase(PhaseCancelled)
	if n := len(r.set.HTTP.Requests()); n != 1 {
		t.Fatalf("requests after cancel = %d, want only the initiate", n)
	}
}

// TestOnlyOneSignInRunsAtATime refuses a second start while the first waits.
func TestOnlyOneSignInRunsAtATime(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.set.HTTP.Script(jsonResponse(200, r.initiateBody(10*time.Minute)))
	if err := r.session.StartSignIn(); err != nil {
		t.Fatalf("StartSignIn: %v", err)
	}
	if err := r.session.StartSignIn(); !errors.Is(err, ErrSignInInProgress) {
		t.Fatalf("second StartSignIn = %v, want ErrSignInInProgress", err)
	}
	r.session.CancelSignIn()
	r.waitForPhase(PhaseCancelled)
}

// TestBrowserFailureIsShownAndCanBeRetried checks that a browser that does not open is a state the view shows, not
// a failed sign-in.
func TestBrowserFailureIsShownAndCanBeRetried(t *testing.T) {
	r := newRig(t, rigOptions{browseErr: errors.New("no browser")})
	r.set.HTTP.Script(jsonResponse(200, r.initiateBody(10*time.Minute)), jsonResponse(200, approvedBody))
	if err := r.session.StartSignIn(); err != nil {
		t.Fatalf("StartSignIn: %v", err)
	}
	pending := r.waitForState("browser failure", func(s Snapshot) bool {
		return s.SignIn.Phase == PhasePending && s.SignIn.BrowserOpenFailed
	})
	if err := r.session.RetryOpenVerificationPage(); err == nil {
		t.Fatal("RetryOpenVerificationPage succeeded with a failing browser")
	}
	if pages := r.waitForBrowsed(2); len(pages) != 2 || pages[1] != pending.SignIn.VerificationURL {
		t.Fatalf("browser calls = %v, want the page opened again on retry", pages)
	}
	r.advancePoll()
	r.waitForPhase(PhaseApproved)
}

// TestKeychainFailureOnApprovalFailsTheSignIn checks that a token the keychain refuses is not reported as signed in.
func TestKeychainFailureOnApprovalFailsTheSignIn(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.set.Keychain.FailNext(fake.OpKeychainSet, errors.New("keychain locked"))
	r.set.HTTP.Script(jsonResponse(200, r.initiateBody(10*time.Minute)), jsonResponse(200, approvedBody))
	if err := r.session.StartSignIn(); err != nil {
		t.Fatalf("StartSignIn: %v", err)
	}
	r.waitForPhase(PhasePending)
	r.advancePoll()
	failed := r.waitForPhase(PhaseFailed)
	if failed.Connected || failed.SignIn.Message == "" {
		t.Fatalf("failed snapshot = %+v, want not connected with a message", failed)
	}
}

// TestSignInWithoutABrowserStillShowsTheCode checks the view has what it needs when the platform has no browser.
func TestSignInWithoutABrowserStillShowsTheCode(t *testing.T) {
	r := newRig(t, rigOptions{noBrowser: true})
	r.set.HTTP.Script(jsonResponse(200, r.initiateBody(10*time.Minute)))
	if err := r.session.StartSignIn(); err != nil {
		t.Fatalf("StartSignIn: %v", err)
	}
	// The code is shown as soon as the backend issues it; the failed browser open is recorded right after, so the
	// test waits for the state to carry both rather than reading the first pending snapshot.
	r.waitForState("the code with a failed browser open", func(s Snapshot) bool {
		return s.SignIn.Phase == PhasePending && s.SignIn.Code != "" && s.SignIn.BrowserOpenFailed
	})
	r.session.CancelSignIn()
	r.waitForPhase(PhaseCancelled)
}

// TestTheTokenNeverReachesLogsStoresOrErrors runs a whole sign-in and then reads every place this package writes:
// the log entries, the account document and the flow state. The keychain is the only place the token may be.
func TestTheTokenNeverReachesLogsStoresOrErrors(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.signIn()
	r.flush()

	document, err := r.set.FS.ReadFile("/config/account.json")
	if err != nil {
		t.Fatalf("read account.json: %v", err)
	}
	surfaces := map[string]string{
		"logs":         r.logText(),
		"account.json": string(document),
		"snapshot":     fmt.Sprintf("%+v", r.session.Snapshot()),
	}
	for name, text := range surfaces {
		if strings.Contains(text, canaryToken) {
			t.Fatalf("%s contains the bearer token", name)
		}
	}
	if !r.eventLogged("auth.signin_approved") {
		t.Fatal("approval was not logged")
	}
}

// TestSignInSurvivesARestart checks the connection comes back from the profile and the keychain.
func TestSignInSurvivesARestart(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.signIn()
	restarted := r.reopen()
	snapshot := restarted.Snapshot()
	if !snapshot.Connected || snapshot.Email != "pat@example.test" {
		t.Fatalf("after restart = %+v, want the connected account", snapshot)
	}
}

// TestApprovedSignInReachesDiskBeforeTheStoreFlushes opens the account document from disk while the running store
// has not flushed. The connection is there at once, so a kill right after approval keeps the sign-in.
func TestApprovedSignInReachesDiskBeforeTheStoreFlushes(t *testing.T) {
	r := newRig(t, rigOptions{})
	r.signIn()
	onDisk, err := store.Open(r.set.FS, fake.NewClock(), "/config")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	profile, err := ProfileDocument(onDisk).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := profile.Connections["https://api.nvm.fyi"]; !ok {
		t.Fatalf("the account document on disk = %+v, want the signed-in account before any flush", profile)
	}
}
