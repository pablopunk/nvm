package auth

import (
	"errors"
	"testing"
	"time"
)

// TestParseDeepLinkReadsOnlySignInLinks covers the accepted shapes and the links the handler must ignore.
func TestParseDeepLinkReadsOnlySignInLinks(t *testing.T) {
	const active = "https://api.nvm.fyi"
	accepted := []struct {
		name, url, wantCode, wantBase string
	}{
		{"bare code", "nvm://auth?code=ABC-123", "ABC-123", active},
		{"production origin", "nvm://auth?code=ABC&base_url=https%3A%2F%2Fapi.nvm.fyi", "ABC", active},
		{"loopback development origin", "nvm://auth?code=ABC&base_url=http%3A%2F%2Flocalhost%3A4321", "ABC", developmentOrigin},
		{"untrusted origin falls back to the active one", "nvm://auth?code=ABC&base_url=https%3A%2F%2Fevil.example", "ABC", active},
		{"reconnect mode", "nvm://auth?code=ABC&mode=reconnect", "ABC", active},
		{"trailing slash on the origin", "nvm://auth?code=ABC&base_url=https%3A%2F%2Fapi.nvm.fyi%2F", "ABC", active},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			link, err := ParseDeepLink(tc.url, active)
			if err != nil {
				t.Fatalf("ParseDeepLink(%q): %v", tc.url, err)
			}
			if link.Code != tc.wantCode || link.BaseURL != tc.wantBase {
				t.Fatalf("link = %+v, want code %q at %q", link, tc.wantCode, tc.wantBase)
			}
		})
	}

	refused := []struct {
		name, url string
		want      error
	}{
		{"other host of the scheme", "nvm://settings?code=ABC", ErrNotAuthLink},
		{"other scheme", "https://auth?code=ABC", ErrNotAuthLink},
		{"not a URL", "::nvm auth", ErrNotAuthLink},
		{"no code", "nvm://auth", ErrMissingCode},
		{"personal access token", "nvm://auth?code=nvm_0123456789", ErrTokenLink},
		{"JWT", "nvm://auth?code=eyJhbGciOiJIUzI1NiJ9", ErrTokenLink},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseDeepLink(tc.url, active); !errors.Is(err, tc.want) {
				t.Fatalf("ParseDeepLink(%q) error = %v, want %v", tc.url, err, tc.want)
			}
		})
	}
}

// TestDeepLinkAttachRegistersTheSchemeAndConsumesTheCode checks the whole path from the OS to a connected account:
// the scheme is claimed, the link's code is exchanged on the poll schedule, and the token is stored.
func TestDeepLinkAttachRegistersTheSchemeAndConsumesTheCode(t *testing.T) {
	r := newRig(t, rigOptions{})
	if _, err := r.session.Attach(r.set.DeepLinks); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if schemes := r.set.DeepLinks.Registered(); len(schemes) != 1 || schemes[0] != Scheme {
		t.Fatalf("registered schemes = %v, want [nvm]", schemes)
	}

	r.set.HTTP.Script(jsonResponse(200, approvedBody))
	r.set.DeepLinks.Deliver("nvm://auth?code=CODE-9")
	r.waitForPhase(PhasePending)
	r.advancePoll()
	approved := r.waitForPhase(PhaseApproved)
	if !approved.Connected || approved.Email != "pat@example.test" {
		t.Fatalf("approved = %+v, want the connected account", approved)
	}
	requests := r.set.HTTP.Requests()
	if len(requests) != 1 || requests[0].URL != "https://api.nvm.fyi/api/auth/device/exchange" {
		t.Fatalf("requests = %+v, want one exchange against production", requests)
	}
	if got := string(requests[0].Body); got != `{"code":"CODE-9"}` {
		t.Fatalf("exchange body = %s, want the link's code", got)
	}
}

// TestDeepLinkDoesNothingWhenItCannotHelp covers the refusals: a token in a link, a link that is not for sign-in,
// and a link while the account is already connected.
func TestDeepLinkDoesNothingWhenItCannotHelp(t *testing.T) {
	r := newRig(t, rigOptions{})
	if _, err := r.session.Attach(r.set.DeepLinks); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	r.set.DeepLinks.Deliver("nvm://auth?code=nvm_leaked")
	r.set.DeepLinks.Deliver("https://www.nvm.fyi/")
	if n := len(r.set.HTTP.Requests()); n != 0 {
		t.Fatalf("refused links made %d requests", n)
	}
	if snapshot := r.session.Snapshot(); snapshot.SignIn.Phase != PhaseIdle {
		t.Fatalf("phase = %s, want idle", snapshot.SignIn.Phase)
	}

	r.signIn()
	r.set.DeepLinks.Deliver("nvm://auth?code=CODE-10")
	if n := len(r.set.HTTP.Requests()); n != 2 {
		t.Fatalf("requests after a link while connected = %d, want the 2 of the first sign-in", n)
	}
}

// TestDeepLinkGivesUpWhenTheCodeExpires checks the exchange window is the code's lifetime: when the window closes
// without approval, the flow ends as expired.
func TestDeepLinkGivesUpWhenTheCodeExpires(t *testing.T) {
	r := newRig(t, rigOptions{})
	if _, err := r.session.Attach(r.set.DeepLinks); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	r.set.HTTP.Script(jsonResponse(200, pendingBody))
	r.set.DeepLinks.Deliver("nvm://auth?code=CODE-11")
	r.waitForPhase(PhasePending)
	r.set.Clock.AwaitTimers(1)
	r.set.Clock.Advance(consumeWindow)
	r.waitForPhase(PhaseExpired)
}

// TestDeepLinkDeniedEndsAsDenied checks an approval refused on the page is reported as refused.
func TestDeepLinkDeniedEndsAsDenied(t *testing.T) {
	r := newRig(t, rigOptions{})
	if _, err := r.session.Attach(r.set.DeepLinks); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	r.set.HTTP.Script(jsonResponse(200, `{"status":"denied"}`))
	r.set.DeepLinks.Deliver("nvm://auth?code=CODE-12")
	r.waitForPhase(PhasePending)
	r.advancePoll()
	r.waitForPhase(PhaseDenied)
}

// TestDeepLinkWindowDurationMatchesTheCodeLifetime pins the window the backend's codes live for.
func TestDeepLinkWindowDurationMatchesTheCodeLifetime(t *testing.T) {
	if consumeWindow != 5*time.Minute || consumePollEvery != 2*time.Second {
		t.Fatalf("window = %s, poll = %s; the device code lives five minutes and is polled every two seconds", consumeWindow, consumePollEvery)
	}
}
