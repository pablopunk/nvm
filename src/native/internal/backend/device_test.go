package backend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// initiateJSON is the initiate response: one code, its verification page, its expiry and the poll interval in
// milliseconds, written as the backend writes it.
func initiateJSON(expiresAt, pollIntervalMs string) string {
	return fmt.Sprintf(`{"code":"CODE-123","verifyUrl":"https://www.nvm.fyi/auth/device?code=CODE-123","expiresAt":%q,"pollIntervalMs":%s}`, expiresAt, pollIntervalMs)
}

// approvedJSON is an exchange response once the user approved the code.
const approvedJSON = `{"status":"ok","token":"tok-approved","user":{"id":"user_1","email":"pat@example.test","role":"member"}}`

// pendingJSON is an exchange response while the user has not approved yet.
const pendingJSON = `{"status":"pending"}`

// signInResult carries a SignIn outcome out of the goroutine that the clock drives.
type signInResult struct {
	approval Approval
	err      error
}

// eventLog records the events SignIn reports, in order.
type eventLog struct {
	events []DeviceEvent
	opened []string
}

// notify records one event.
func (l *eventLog) notify(event DeviceEvent) {
	l.events = append(l.events, event)
}

// open records the URL the browser would open.
func (l *eventLog) open(url string) {
	l.opened = append(l.opened, url)
}

// phases returns the phase of every recorded event.
func (l *eventLog) phases() []DevicePhase {
	phases := make([]DevicePhase, 0, len(l.events))
	for _, event := range l.events {
		phases = append(phases, event.Phase)
	}
	return phases
}

// runSignIn runs SignIn with inline waits, so each poll interval moves the fake clock in the calling goroutine.
func runSignIn(t *testing.T, h harness, log *eventLog) signInResult {
	t.Helper()
	h.useSynchronousWaits()
	approval, err := h.client.SignIn(context.Background(), log.open, log.notify)
	return signInResult{approval: approval, err: err}
}

// TestStartDeviceSignInReadsTheCodeAndInterval checks the initiate contract, including the one-second floor on
// the poll interval.
func TestStartDeviceSignInReadsTheCodeAndInterval(t *testing.T) {
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, initiateJSON(epochPlus(fiveMinutes), "500")))
	sign, err := h.client.StartDeviceSignIn(context.Background())
	if err != nil {
		t.Fatalf("StartDeviceSignIn: %v", err)
	}
	if sign.Code != "CODE-123" || sign.Interval != time.Second || !strings.HasPrefix(sign.VerificationURL, "https://www.nvm.fyi/auth/device") {
		t.Fatalf("sign-in = %+v, want the code, the page and a 1s interval", sign)
	}
}

// TestStartDeviceSignInDefaultsTheInterval checks a response without pollIntervalMs uses the two-second default.
func TestStartDeviceSignInDefaultsTheInterval(t *testing.T) {
	h := newHarness(t)
	body := `{"code":"C","verifyUrl":"https://www.nvm.fyi/auth/device","expiresAt":"` + epochPlus(fiveMinutes) + `"}`
	h.http.Script(jsonBody(http.StatusOK, body))
	sign, err := h.client.StartDeviceSignIn(context.Background())
	if err != nil {
		t.Fatalf("StartDeviceSignIn: %v", err)
	}
	if sign.Interval != defaultDevicePollEvery {
		t.Fatalf("Interval = %v, want %v", sign.Interval, defaultDevicePollEvery)
	}
}

// TestStartDeviceSignInRejectsUnsafeResponses checks a response the client cannot trust fails before the browser
// is sent anywhere: missing code, a non-https page, a bad expiry, or a code already expired.
func TestStartDeviceSignInRejectsUnsafeResponses(t *testing.T) {
	future := epochPlus(fiveMinutes)
	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{name: "missing code", body: `{"code":"","verifyUrl":"https://www.nvm.fyi/a","expiresAt":"` + future + `"}`},
		{name: "plain http page", body: `{"code":"C","verifyUrl":"http://evil.example/a","expiresAt":"` + future + `"}`},
		{name: "other scheme", body: `{"code":"C","verifyUrl":"javascript:alert(1)","expiresAt":"` + future + `"}`},
		{name: "bad expiry", body: `{"code":"C","verifyUrl":"https://www.nvm.fyi/a","expiresAt":"tomorrow"}`},
		{name: "already expired", body: `{"code":"C","verifyUrl":"https://www.nvm.fyi/a","expiresAt":"` + epochPlus(-time.Minute) + `"}`, wantErr: ErrSignInExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.http.Script(jsonBody(http.StatusOK, tt.body))
			_, err := h.client.StartDeviceSignIn(context.Background())
			if err == nil {
				t.Fatal("StartDeviceSignIn succeeded, want an error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && !IsKind(err, KindBadResponse) {
				t.Fatalf("error = %v, want KindBadResponse", err)
			}
		})
	}
}

// TestSignInApprovesAfterPendingPolls checks the happy path: open the page, poll while pending, and return the
// token and account once approved, with the events a UI would show in order.
func TestSignInApprovesAfterPendingPolls(t *testing.T) {
	h := newHarness(t)
	h.http.Script(
		jsonBody(http.StatusOK, initiateJSON(epochPlus(fiveMinutes), "2000")),
		jsonBody(http.StatusOK, pendingJSON),
		jsonBody(http.StatusOK, pendingJSON),
		jsonBody(http.StatusOK, approvedJSON),
	)
	log := &eventLog{}
	result := runSignIn(t, h, log)
	if result.err != nil {
		t.Fatalf("SignIn: %v", result.err)
	}
	if result.approval.Token.Reveal() != "tok-approved" || result.approval.User.Email != "pat@example.test" {
		t.Fatalf("approval = %+v, want the approved token and account", result.approval)
	}
	if len(log.opened) != 1 || !strings.Contains(log.opened[0], "code=CODE-123") {
		t.Fatalf("opened = %v, want the verification page once", log.opened)
	}
	want := []DevicePhase{DevicePending, DeviceApproved}
	if got := log.phases(); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("phases = %v, want %v", got, want)
	}
	if n := len(h.http.Requests()); n != 4 {
		t.Fatalf("made %d requests, want initiate plus three exchanges", n)
	}
}

// TestSignInBacksOffWhenTheBackendSlowsDown checks a 429 widens the poll interval by the RFC 8628 step and reports
// the new interval, and that polling then still succeeds.
func TestSignInBacksOffWhenTheBackendSlowsDown(t *testing.T) {
	h := newHarness(t)
	h.http.Script(
		jsonBody(http.StatusOK, initiateJSON(epochPlus(fiveMinutes), "2000")),
		fake.Response{Status: http.StatusTooManyRequests, Body: []byte(`{"error":{"type":"rate_limited","message":"slow"}}`)},
		jsonBody(http.StatusOK, approvedJSON),
	)
	log := &eventLog{}
	result := runSignIn(t, h, log)
	if result.err != nil {
		t.Fatalf("SignIn: %v", result.err)
	}
	var slowed *DeviceEvent
	for i := range log.events {
		if log.events[i].Phase == DeviceSlowDown {
			slowed = &log.events[i]
		}
	}
	if slowed == nil || slowed.Interval != 2*time.Second+slowDownStep {
		t.Fatalf("slow-down event = %+v, want the interval widened by %v", slowed, slowDownStep)
	}
}

// TestSignInReportsDenial checks a refused approval ends the flow with ErrSignInDenied.
func TestSignInReportsDenial(t *testing.T) {
	h := newHarness(t)
	h.http.Script(
		jsonBody(http.StatusOK, initiateJSON(epochPlus(fiveMinutes), "2000")),
		jsonBody(http.StatusOK, `{"status":"denied"}`),
	)
	log := &eventLog{}
	result := runSignIn(t, h, log)
	if !errors.Is(result.err, ErrSignInDenied) {
		t.Fatalf("SignIn error = %v, want ErrSignInDenied", result.err)
	}
	if last := log.phases(); last[len(last)-1] != DeviceDenied {
		t.Fatalf("phases = %v, want to end with denied", last)
	}
}

// TestSignInStopsWhenTheCodeIsGone checks a 410 from the exchange ends the flow as expired, the same as the
// desktop app treats it.
func TestSignInStopsWhenTheCodeIsGone(t *testing.T) {
	h := newHarness(t)
	h.http.Script(
		jsonBody(http.StatusOK, initiateJSON(epochPlus(fiveMinutes), "2000")),
		jsonBody(http.StatusGone, `{"status":"consumed"}`),
	)
	result := runSignIn(t, h, &eventLog{})
	if !errors.Is(result.err, ErrSignInExpired) {
		t.Fatalf("SignIn error = %v, want ErrSignInExpired", result.err)
	}
}

// TestSignInExpiresOnTheClockWithoutAnotherPoll checks the code's expiry is judged on the clock: once the deadline
// passes between polls, the flow ends without asking the backend again.
func TestSignInExpiresOnTheClockWithoutAnotherPoll(t *testing.T) {
	h := newHarness(t)
	h.http.Script(
		jsonBody(http.StatusOK, initiateJSON(epochPlus(3*time.Second), "2000")),
		jsonBody(http.StatusOK, pendingJSON),
	)
	log := &eventLog{}
	result := runSignIn(t, h, log)
	if !errors.Is(result.err, ErrSignInExpired) {
		t.Fatalf("SignIn error = %v, want ErrSignInExpired", result.err)
	}
	if n := len(h.http.Requests()); n != 2 {
		t.Fatalf("made %d requests, want initiate and one poll before expiry", n)
	}
}

// TestSignInKeepsPollingThroughNetworkBlips checks an offline poll does not end the flow: the next poll still
// reaches the approval.
func TestSignInKeepsPollingThroughNetworkBlips(t *testing.T) {
	h := newHarness(t)
	h.http.Script(
		jsonBody(http.StatusOK, initiateJSON(epochPlus(fiveMinutes), "2000")),
		fake.Response{Err: errNetworkDown},
		jsonBody(http.StatusOK, approvedJSON),
	)
	result := runSignIn(t, h, &eventLog{})
	if result.err != nil || result.approval.Token.Reveal() != "tok-approved" {
		t.Fatalf("SignIn = %+v, %v; want approval after a network blip", result.approval, result.err)
	}
}

// TestSignInStopsOnAServerError checks a server refusal, such as the device kill switch, ends the flow instead of
// polling until expiry.
func TestSignInStopsOnAServerError(t *testing.T) {
	h := newHarness(t)
	h.http.Script(
		jsonBody(http.StatusOK, initiateJSON(epochPlus(fiveMinutes), "2000")),
		fake.Response{Status: http.StatusServiceUnavailable, Body: []byte(`{"error":{"type":"service_unavailable","message":"off"}}`)},
	)
	result := runSignIn(t, h, &eventLog{})
	if !IsKind(result.err, KindServer) {
		t.Fatalf("SignIn error = %v, want KindServer", result.err)
	}
}

// TestPollRejectsApprovalWithoutToken checks an approved response that lacks its token is not accepted.
func TestPollRejectsApprovalWithoutToken(t *testing.T) {
	h := newHarness(t)
	h.http.Script(jsonBody(http.StatusOK, `{"status":"ok","user":{"email":"a@example.test"}}`))
	_, err := h.client.PollDeviceSignIn(context.Background(), "CODE-123")
	if !IsKind(err, KindBadResponse) {
		t.Fatalf("PollDeviceSignIn error = %v, want KindBadResponse", err)
	}
}

// TestSignInCancelledBeforeStartPollsNothing checks a cancelled sign-in reports cancellation and never polls. At
// most the initiate attempt reaches the transport, and it fails on the cancelled context.
func TestSignInCancelledBeforeStartPollsNothing(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.client.SignIn(ctx, nil, nil)
	if !IsKind(err, KindCanceled) {
		t.Fatalf("SignIn error = %v, want KindCanceled", err)
	}
	if n := len(h.http.Requests()); n > 1 {
		t.Fatalf("made %d requests after cancellation, want at most the initiate attempt", n)
	}
}

// TestApprovalTokenPrintsRedacted checks the approved token never appears in formatted output, so a struct logged
// or printed by accident does not leak the credential.
func TestApprovalTokenPrintsRedacted(t *testing.T) {
	approval := Approval{Token: Token("tok-approved")}
	if got := fmt.Sprintf("%v %+v %#v", approval, approval, approval); strings.Contains(got, "tok-approved") {
		t.Fatalf("formatted approval leaks the token: %s", got)
	}
}
