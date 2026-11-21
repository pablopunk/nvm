package account

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/backend/auth"
)

// State is one named state of the account page, for goldens and tests.
type State struct {
	Name     string
	Snapshot auth.Snapshot
}

// accountCode and accountURL are the one-time code and verification page the sign-in fixtures show.
const (
	accountCode = "CODE-123"
	accountURL  = "https://www.nvm.fyi/auth/device?code=CODE-123"
)

// accountExpiry is the instant the fixture code expires. It is fixed so the goldens never drift with the clock.
var accountExpiry = time.Date(2026, time.November, 4, 10, 10, 0, 0, time.UTC)

// States lists every state the account page must render, in the order the goldens are written.
func States() []State {
	signedInAccount := auth.Snapshot{
		Environment: backend.EnvironmentProduction,
		BaseURL:     "https://api.nvm.fyi",
		Connected:   true,
		Email:       "pat@example.test",
		Role:        "member",
		UserID:      "user_1",
		SignIn:      auth.SignInState{Phase: auth.PhaseApproved},
		Account:     auth.AccountInfo{Plan: "pro", Balance: 1200, HasBalance: true},
	}
	withNotice := signedInAccount
	withNotice.Account = auth.AccountInfo{Plan: "free", Balance: 0, HasBalance: true}
	withNotice.Notice = &auth.Notice{
		Kind:    "free_spend_cap_reached",
		Title:   "Free AI temporarily paused",
		Message: "Free-pool requests are temporarily paused. Capacity may return earlier as in-flight reservations settle or release.",
	}
	withBYO := signedInAccount
	withBYO.BYO = &auth.BYOSetting{Provider: "openai", ModelID: "example-model", BaseURL: "https://api.openai.example/v1"}
	signedOut := auth.Snapshot{Environment: backend.EnvironmentProduction, BaseURL: "https://api.nvm.fyi", SignIn: auth.SignInState{Phase: auth.PhaseIdle}}
	pending := signedOut
	pending.SignIn = auth.SignInState{Phase: auth.PhasePending, Code: accountCode, VerificationURL: accountURL, ExpiresAt: accountExpiry}
	browserFailed := pending
	browserFailed.SignIn.BrowserOpenFailed = true

	return []State{
		{Name: "signed-out", Snapshot: signedOut},
		{Name: "starting", Snapshot: withPhase(signedOut, auth.PhaseStarting)},
		{Name: "pending", Snapshot: pending},
		{Name: "pending-browser-failed", Snapshot: browserFailed},
		{Name: "denied", Snapshot: withPhase(signedOut, auth.PhaseDenied)},
		{Name: "expired", Snapshot: withPhase(signedOut, auth.PhaseExpired)},
		{Name: "cancelled", Snapshot: withPhase(signedOut, auth.PhaseCancelled)},
		{Name: "failed", Snapshot: failedSignIn(signedOut)},
		{Name: "signed-in", Snapshot: signedInAccount},
		{Name: "signed-in-byo-key", Snapshot: withBYO},
		{Name: "signed-in-spend-cap", Snapshot: withNotice},
	}
}

// withPhase returns s with the sign-in in phase.
func withPhase(s auth.Snapshot, phase auth.Phase) auth.Snapshot {
	s.SignIn = auth.SignInState{Phase: phase}
	return s
}

// failedSignIn returns s with a failed sign-in and the words the view shows for it.
func failedSignIn(s auth.Snapshot) auth.Snapshot {
	s.SignIn = auth.SignInState{Phase: auth.PhaseFailed, Message: "Unable to start sign-in. Check your connection and try again."}
	return s
}
