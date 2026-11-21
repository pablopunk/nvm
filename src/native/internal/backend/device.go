package backend

import (
	"context"
	"encoding/json"
	"net/url"
	"time"
)

// Device sign-in timing. The backend suggests a poll interval; the client never polls faster than the floor,
// and backs off by the RFC 8628 step when the backend answers 429.
const (
	deviceTimeout          = 10 * time.Second
	defaultDevicePollEvery = 2 * time.Second
	minDevicePollEvery     = time.Second
	slowDownStep           = 5 * time.Second
)

// DevicePhase is one state of a device sign-in.
type DevicePhase string

// The device sign-in phases. Pending and SlowDown may repeat; the others end the flow.
const (
	// DevicePending means the user has not approved the code yet.
	DevicePending DevicePhase = "pending"
	// DeviceApproved means the user approved the code and a token was issued.
	DeviceApproved DevicePhase = "approved"
	// DeviceDenied means the approval was refused.
	DeviceDenied DevicePhase = "denied"
	// DeviceExpired means the code expired or was used before approval.
	DeviceExpired DevicePhase = "expired"
	// DeviceSlowDown means the backend asked the client to poll less often.
	DeviceSlowDown DevicePhase = "slow_down"
)

// DeviceSignIn is a started device sign-in: the code the user enters on the verification page, the page URL,
// when the code expires and how often to poll. The backend issues a single code, used both to show the user
// and to exchange for a token.
type DeviceSignIn struct {
	// Code is shown to the user and passed to PollDeviceSignIn.
	Code string
	// VerificationURL is the page the user opens to approve the code. It is https, or loopback http in development.
	VerificationURL string
	// ExpiresAt is the moment the code stops working.
	ExpiresAt time.Time
	// Interval is the minimum wait between polls.
	Interval time.Duration
}

// DeviceUser identifies the account a device sign-in approved.
type DeviceUser struct {
	// ID is the backend user identifier, when the backend sent one.
	ID string
	// Email is the account email.
	Email string
	// Role is the account role.
	Role string
}

// DevicePoll is the result of one poll. Token is set only for DeviceApproved.
type DevicePoll struct {
	// Phase is the state the poll observed.
	Phase DevicePhase
	// Token is the bearer token issued on approval.
	Token Token
	// User is the approved account.
	User DeviceUser
}

// Approval is a completed device sign-in. The caller persists Token in the keychain; this package does not.
type Approval struct {
	// Token is the bearer token for the approved account.
	Token Token
	// User is the approved account.
	User DeviceUser
}

// deviceInitiateWire is the response of the initiate route.
type deviceInitiateWire struct {
	Code           string `json:"code"`
	VerifyURL      string `json:"verifyUrl"`
	ExpiresAt      string `json:"expiresAt"`
	PollIntervalMs *int   `json:"pollIntervalMs"`
}

// deviceExchangeWire is the response of the exchange route while or after the code is polled.
type deviceExchangeWire struct {
	Status string    `json:"status"`
	Token  string    `json:"token"`
	User   *userWire `json:"user"`
}

// userWire is the approved account inside an exchange response.
type userWire struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// StartDeviceSignIn asks the backend for a device code. It sends the client label and no token, since the
// sign-in creates the token. The call is a POST and is not retried.
func (c *Client) StartDeviceSignIn(ctx context.Context) (DeviceSignIn, error) {
	body, err := json.Marshal(map[string]string{"label": c.deviceLabel})
	if err != nil {
		return DeviceSignIn{}, &Error{Kind: KindRejected, Message: "could not encode the sign-in request", Cause: err}
	}
	var wire deviceInitiateWire
	err = c.callJSON(ctx, requestSpec{
		method:  "POST",
		path:    pathDeviceInitiate,
		body:    body,
		auth:    authNone,
		timeout: deviceTimeout,
	}, &wire)
	if err != nil {
		return DeviceSignIn{}, err
	}
	return wire.signIn(c.clock.Now())
}

// signIn checks the initiate response and converts it. now is only used to reject a code that is already
// expired, which would make the flow fail on its first poll.
func (w deviceInitiateWire) signIn(now time.Time) (DeviceSignIn, error) {
	if w.Code == "" {
		return DeviceSignIn{}, unexpectedDeviceShape("the sign-in code is missing")
	}
	verifyURL, err := validVerificationURL(w.VerifyURL)
	if err != nil {
		return DeviceSignIn{}, err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, w.ExpiresAt)
	if err != nil {
		return DeviceSignIn{}, unexpectedDeviceShape("the expiry is not a timestamp")
	}
	if !expiresAt.After(now) {
		return DeviceSignIn{}, ErrSignInExpired
	}
	interval := defaultDevicePollEvery
	if w.PollIntervalMs != nil {
		interval = max(minDevicePollEvery, time.Duration(*w.PollIntervalMs)*time.Millisecond)
	}
	return DeviceSignIn{Code: w.Code, VerificationURL: verifyURL, ExpiresAt: expiresAt, Interval: interval}, nil
}

// validVerificationURL accepts only https pages, or loopback http for development, so the client never opens
// an arbitrary scheme the backend names.
func validVerificationURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", unexpectedDeviceShape("the verification URL is not an absolute URL")
	}
	host := parsed.Hostname()
	if parsed.Scheme == "https" || (parsed.Scheme == "http" && isLoopbackHost(host)) {
		return raw, nil
	}
	return "", unexpectedDeviceShape("the verification URL must use https")
}

// PollDeviceSignIn checks the code once. It maps 410 to DeviceExpired and 429 to DeviceSlowDown, and returns
// an error for network failures and for responses the client does not understand. Transient network errors
// are returned as KindOffline or KindTimeout so SignIn can keep polling.
func (c *Client) PollDeviceSignIn(ctx context.Context, code string) (DevicePoll, error) {
	body, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return DevicePoll{}, &Error{Kind: KindRejected, Message: "could not encode the poll request", Cause: err}
	}
	ex, err := c.send(ctx, requestSpec{
		method:  "POST",
		path:    pathDeviceExchange,
		body:    body,
		auth:    authNone,
		timeout: deviceTimeout,
	})
	if err != nil {
		return DevicePoll{}, err
	}
	defer ex.release()
	data, _, readErr := readCapped(ex.body, maxResponseBytes)
	if readErr != nil {
		return DevicePoll{}, c.readFailure(ctx, ex, readErr)
	}
	switch {
	case ex.status == 410:
		return DevicePoll{Phase: DeviceExpired}, nil
	case ex.status == 429:
		return DevicePoll{Phase: DeviceSlowDown}, nil
	case ex.status < 200 || ex.status > 299:
		return DevicePoll{}, errorFromResponse(ex.status, ex.retryAfter, truncate(data, maxErrorBytes))
	}
	var wire deviceExchangeWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return DevicePoll{}, &Error{Kind: KindBadResponse, Status: ex.status, Message: "poll response is not JSON", Cause: err}
	}
	return wire.poll(ex.status)
}

// poll converts an exchange response to a phase. An approval must carry both a token and a user.
func (w deviceExchangeWire) poll(status int) (DevicePoll, error) {
	switch w.Status {
	case "pending":
		return DevicePoll{Phase: DevicePending}, nil
	case "denied":
		return DevicePoll{Phase: DeviceDenied}, nil
	case "expired", "consumed":
		return DevicePoll{Phase: DeviceExpired}, nil
	case "ok":
		if w.Token == "" || w.User == nil || w.User.Email == "" {
			return DevicePoll{}, &Error{Kind: KindBadResponse, Status: status, Message: "approved poll response lacks a token or user"}
		}
		return DevicePoll{
			Phase: DeviceApproved,
			Token: Token(w.Token),
			User:  DeviceUser{ID: w.User.ID, Email: w.User.Email, Role: w.User.Role},
		}, nil
	default:
		return DevicePoll{}, &Error{Kind: KindBadResponse, Status: status, Message: "poll response has an unknown status"}
	}
}

// unexpectedDeviceShape is the error for a device response the client cannot trust.
func unexpectedDeviceShape(detail string) *Error {
	return &Error{Kind: KindBadResponse, Status: 200, Message: "device sign-in response: " + detail}
}
