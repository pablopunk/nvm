package backend

import (
	"errors"
	"fmt"
	"time"
)

// Kind classifies a failed backend call. Callers branch on Kind and show UserMessage; they never parse
// server messages.
type Kind string

// The failure kinds. Offline and Timeout mean the backend could not be reached in time; Canceled means the
// caller's context ended first.
const (
	// KindOffline means the connection failed before a response arrived.
	KindOffline Kind = "offline"
	// KindTimeout means a request deadline or the caller's deadline passed before a response arrived.
	KindTimeout Kind = "timeout"
	// KindCanceled means the caller cancelled the call.
	KindCanceled Kind = "canceled"
	// KindUnauthorized means the token is missing, expired or revoked (HTTP 401).
	KindUnauthorized Kind = "unauthorized"
	// KindInsufficientCredits means the account cannot pay for the request (HTTP 402). CreditsInfo is set.
	KindInsufficientCredits Kind = "insufficient_credits"
	// KindRateLimited means the backend refused for too many requests (HTTP 429). RetryAfter may be set.
	KindRateLimited Kind = "rate_limited"
	// KindServer means the backend failed or is temporarily unavailable (HTTP 5xx, including kill switches).
	KindServer Kind = "server"
	// KindIncompatible means this desktop client is too old or uses an API version the backend retired.
	KindIncompatible Kind = "incompatible"
	// KindRejected means the backend refused the request for another reason (other HTTP 4xx).
	KindRejected Kind = "rejected"
	// KindBadResponse means the backend answered with something this client cannot read.
	KindBadResponse Kind = "bad_response"
)

// CreditsInfo is the balance information carried by an insufficient-credits refusal.
type CreditsInfo struct {
	// EstimatedCredits is what the refused request would have cost, when the backend said.
	EstimatedCredits int
	// Balance is the credit balance the backend reported as available.
	Balance int
}

// Error is the one error type the backend client returns. Message is for logs and diagnostics; show
// UserMessage to people.
type Error struct {
	// Kind classifies the failure.
	Kind Kind
	// Status is the HTTP status, or zero when no response arrived.
	Status int
	// Code is the backend's machine-readable error type, such as free_spend_cap_reached, when it sent one.
	Code string
	// Message is a short developer-facing description. It never contains the token.
	Message string
	// Retryable reports whether trying the same call again later may succeed.
	Retryable bool
	// CreditsInfo is set for KindInsufficientCredits.
	CreditsInfo *CreditsInfo
	// RetryAfter is the wait the backend asked for, set for KindRateLimited when known.
	RetryAfter time.Duration
	// MinimumVersion is the oldest desktop version the backend accepts, set for KindIncompatible when known.
	MinimumVersion string
	// UpdateURL is where the user gets an update, set for KindIncompatible when the backend sent one.
	UpdateURL string
	// Cause is the underlying transport or decoding error, if any.
	Cause error
}

// Error describes the failure for logs.
func (e *Error) Error() string {
	if e.Status == 0 {
		return fmt.Sprintf("backend: %s: %s", e.Kind, e.Message)
	}
	return fmt.Sprintf("backend: %s (HTTP %d): %s", e.Kind, e.Status, e.Message)
}

// Unwrap returns the underlying cause, so errors.Is can see context and network errors.
func (e *Error) Unwrap() error {
	return e.Cause
}

// ErrSignInExpired is returned by SignIn when the device code expired or was used before approval.
var ErrSignInExpired = errors.New("backend: device sign-in code expired")

// ErrSignInDenied is returned by SignIn when the approval was refused.
var ErrSignInDenied = errors.New("backend: device sign-in was denied")

// IsKind reports whether err is a *Error of the given kind.
func IsKind(err error, kind Kind) bool {
	typed := asError(err)
	return typed != nil && typed.Kind == kind
}

// asError returns the *Error inside err, or nil when err holds none.
func asError(err error) *Error {
	var typed *Error
	if errors.As(err, &typed) {
		return typed
	}
	return nil
}
