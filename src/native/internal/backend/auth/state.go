package auth

import (
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// Phase is the state of the sign-in flow the palette shows.
type Phase string

// The sign-in phases. Starting and pending are the only ones in which a flow is running.
const (
	// PhaseIdle means no sign-in has run since the app started.
	PhaseIdle Phase = "idle"
	// PhaseStarting means the device code is being requested.
	PhaseStarting Phase = "starting"
	// PhasePending means the code is shown and the user is approving it in the browser.
	PhasePending Phase = "pending"
	// PhaseApproved means the account is connected.
	PhaseApproved Phase = "approved"
	// PhaseDenied means the approval was refused.
	PhaseDenied Phase = "denied"
	// PhaseExpired means the code expired or was used before approval.
	PhaseExpired Phase = "expired"
	// PhaseCancelled means the user stopped the flow.
	PhaseCancelled Phase = "cancelled"
	// PhaseFailed means the flow could not run; Message says why in words the user can act on.
	PhaseFailed Phase = "failed"
)

// SignInState is what the sign-in view shows.
type SignInState struct {
	// Phase is the state of the flow.
	Phase Phase
	// Code is the one-time code the user compares with the verification page. Set in PhasePending.
	Code string
	// VerificationURL is the page the user approves the code on. Set in PhasePending.
	VerificationURL string
	// ExpiresAt is when the code stops working. Set in PhasePending.
	ExpiresAt time.Time
	// BrowserOpenFailed reports that the browser did not open the verification page.
	BrowserOpenFailed bool
	// Message is the words for PhaseFailed.
	Message string
}

// AccountInfo holds the account details the view shows when the backend reports them.
type AccountInfo struct {
	// Plan is the plan name, when the backend reports it.
	Plan string
	// Balance is the total credit balance, when the backend reports it.
	Balance int
	// HasBalance reports whether Balance was read.
	HasBalance bool
	// Error is the reason the details could not be read, empty when they were read or not asked for.
	Error string
}

// Snapshot is everything the account view draws, read in one lock. It is a copy; the view never holds the session.
type Snapshot struct {
	// Environment and BaseURL name the backend the app is connected to.
	Environment backend.EnvironmentKind
	BaseURL     string
	// Connected reports whether a token is stored for BaseURL.
	Connected bool
	// Email, Role and UserID identify the signed-in account. Empty when not connected.
	Email  string
	Role   string
	UserID string
	// SignIn is the state of the sign-in flow.
	SignIn SignInState
	// Account holds the details refreshed from the backend.
	Account AccountInfo
	// BYO describes the bring-your-own key, or nil when none is set.
	BYO *BYOSetting
	// Notice is the spend-cap notice to show, or nil.
	Notice *Notice
}
