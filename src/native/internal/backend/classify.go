package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// HTTP statuses the client branches on. Other statuses fall into the 4xx and 5xx ranges below.
const (
	statusUnauthorized    = 401
	statusPaymentRequired = 402
	statusNotFound        = 404
	statusTooManyRequests = 429
	statusUpgradeRequired = 426
)

// Error types the client branches on or words specially, as the backend's contract fixtures name them.
const (
	codeUnsupportedClient        = "unsupported_client"
	codeInsufficientCredits      = "insufficient_credits"
	codeRateLimited              = "rate_limited"
	codeFreeSpendCapReached      = "free_spend_cap_reached"
	codeModelNotConfigured       = "model_not_configured"
	codePricingUnavailable       = "pricing_unavailable"
	codeDecisionModelUnavailable = "decision_model_unavailable"
	codeUnsupportedRequestShape  = "unsupported_request_shape"
	codeRequestTooLarge          = "request_too_large"
	codeInvalidDecisionRequest   = "invalid_decision_request"
	codeDecisionProviderInvalid  = "decision_provider_response_invalid"
)

// maxMessageLength bounds the server message kept on an Error, so a large body never reaches logs whole.
const maxMessageLength = 300

// envelope is the machine-readable error body the backend sends: {"error": {...}}. Only the fields the
// client branches on are read; unknown fields are ignored.
type envelope struct {
	Type           string `json:"type"`
	Message        string `json:"message"`
	Estimated      *int   `json:"estimated_credits"`
	Balance        *int   `json:"balance"`
	RetryAfter     *int   `json:"retry_after"`
	MinimumDesktop string `json:"minimum_supported_desktop_version"`
	UpdateURL      string `json:"update_url"`
}

// envelopeBody wraps the envelope under its "error" key.
type envelopeBody struct {
	Error *envelope `json:"error"`
}

// parseEnvelope returns the error envelope in body, and false when the body is not one.
func parseEnvelope(body []byte) (envelope, bool) {
	var wrapped envelopeBody
	if err := json.Unmarshal(body, &wrapped); err != nil || wrapped.Error == nil {
		return envelope{}, false
	}
	return *wrapped.Error, true
}

// errorFromResponse classifies a non-success HTTP response. body is the bounded response body and
// retryAfter the Retry-After header, if any.
func errorFromResponse(status int, retryAfter string, body []byte) *Error {
	env, ok := parseEnvelope(body)
	if !ok {
		env = envelope{Message: fmt.Sprintf("HTTP %d without an error envelope", status)}
	}
	return errorFromEnvelope(status, env, retryAfter)
}

// errorFromEnvelope maps an error envelope to a Kind. Status is zero for an error event inside a stream,
// where the HTTP status was already 200.
func errorFromEnvelope(status int, env envelope, retryAfter string) *Error {
	err := &Error{Status: status, Code: env.Type, Message: boundedMessage(env.Message, status)}
	switch {
	case env.Type == codeUnsupportedClient || status == statusUpgradeRequired:
		err.Kind = KindIncompatible
		err.MinimumVersion = env.MinimumDesktop
		err.UpdateURL = env.UpdateURL
	case status == statusUnauthorized:
		err.Kind = KindUnauthorized
	case status == statusPaymentRequired || env.Type == codeInsufficientCredits:
		err.Kind = KindInsufficientCredits
		err.CreditsInfo = &CreditsInfo{EstimatedCredits: valueOrZero(env.Estimated), Balance: valueOrZero(env.Balance)}
	case status == statusTooManyRequests || env.Type == codeRateLimited:
		err.Kind = KindRateLimited
		err.Retryable = true
		err.RetryAfter = retryAfterDuration(retryAfter, env.RetryAfter)
	case status >= 400 && status < 500:
		err.Kind = KindRejected
	default:
		err.Kind = KindServer
		err.Retryable = true
	}
	return err
}

// boundedMessage trims a server message and substitutes a generic one when the server sent none.
func boundedMessage(message string, status int) string {
	message = strings.TrimSpace(message)
	if message == "" {
		if status == 0 {
			return "the stream reported an error"
		}
		return fmt.Sprintf("HTTP %d", status)
	}
	if len(message) > maxMessageLength {
		return message[:maxMessageLength] + "..."
	}
	return message
}

// retryAfterDuration reads a Retry-After header in seconds, falling back to the body's retry_after.
func retryAfterDuration(header string, bodySeconds *int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if bodySeconds != nil && *bodySeconds > 0 {
		return time.Duration(*bodySeconds) * time.Second
	}
	return 0
}

// valueOrZero dereferences an optional integer, using zero when the backend omitted it.
func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

// classifyTransport maps a failed Do or body read to a Kind. timedOut reports that the client's own request
// deadline fired; caller is the context the caller passed in.
func classifyTransport(cause error, timedOut bool, caller context.Context) *Error {
	err := &Error{Cause: cause, Message: "could not reach the Nevermind backend"}
	switch {
	case timedOut || errors.Is(cause, context.DeadlineExceeded) || errors.Is(caller.Err(), context.DeadlineExceeded):
		err.Kind = KindTimeout
		err.Message = "the Nevermind backend did not answer in time"
		err.Retryable = true
	case errors.Is(cause, context.Canceled) || errors.Is(caller.Err(), context.Canceled):
		err.Kind = KindCanceled
		err.Message = "the call was cancelled"
	case isNetTimeout(cause):
		err.Kind = KindTimeout
		err.Message = "the Nevermind backend did not answer in time"
		err.Retryable = true
	default:
		err.Kind = KindOffline
		err.Retryable = true
	}
	return err
}

// isNetTimeout reports whether cause is a network timeout.
func isNetTimeout(cause error) bool {
	var netErr net.Error
	return errors.As(cause, &netErr) && netErr.Timeout()
}

// isNetworkKind reports whether a failure means the backend could not be reached, the signal the offline
// tracker counts.
func isNetworkKind(kind Kind) bool {
	return kind == KindOffline || kind == KindTimeout
}
