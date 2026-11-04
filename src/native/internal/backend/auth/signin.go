package auth

import (
	"context"
	"errors"
	"time"

	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/safego"
)

// ErrSignInInProgress is returned when a sign-in starts while another one is running.
var ErrSignInInProgress = errors.New("auth: a sign-in is already running")

// ErrNoVerificationPage is returned when the verification page is asked for before a code exists.
var ErrNoVerificationPage = errors.New("auth: no verification page is waiting for approval")

// signInFailedMessage is what the view says when the backend could not start the sign-in.
const signInFailedMessage = "Unable to start sign-in. Check your connection and try again."

// keychainFailedMessage is what the view says when the approved token could not be stored.
const keychainFailedMessage = "Signed in, but the keychain could not keep the token. Try again."

// StartSignIn starts a device sign-in against the active backend. It returns at once; progress arrives through
// Snapshot and OnChange, and the browser opens the verification page.
func (s *Session) StartSignIn() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if !s.beginFlowLocked(cancel) {
		s.mu.Unlock()
		cancel()
		return ErrSignInInProgress
	}
	client := s.client
	s.mu.Unlock()
	s.notifyChange()
	safego.Go("auth.signin", func(context.Context) { s.runSignIn(ctx, client) })
	return nil
}

// CancelSignIn stops the running sign-in. The flow ends as cancelled when its wait returns; calling it with no
// running flow does nothing.
func (s *Session) CancelSignIn() {
	s.mu.Lock()
	cancel := s.cancelSignIn
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// RetryOpenVerificationPage opens the verification page of the waiting code again, for when the browser did not
// open the first time.
func (s *Session) RetryOpenVerificationPage() error {
	s.mu.Lock()
	url := s.sign.VerificationURL
	s.mu.Unlock()
	if url == "" {
		return ErrNoVerificationPage
	}
	err := s.openPage(url)
	s.mu.Lock()
	s.sign.BrowserOpenFailed = err != nil
	s.mu.Unlock()
	s.notifyChange()
	return err
}

// beginFlowLocked claims the flow for a new sign-in and records its cancel function. It fails while a flow is
// starting or waiting. It must run with s.mu held.
func (s *Session) beginFlowLocked(cancel context.CancelFunc) bool {
	if s.sign.Phase == PhaseStarting || s.sign.Phase == PhasePending {
		return false
	}
	s.cancelSignIn = cancel
	s.sign = SignInState{Phase: PhaseStarting}
	return true
}

// runSignIn runs the device flow on the client it was started with and records how it ended.
func (s *Session) runSignIn(ctx context.Context, client *backend.Client) {
	approval, err := client.SignIn(ctx, s.openVerificationPage, s.recordDeviceEvent)
	if err != nil {
		s.endFlowFailed(ctx, client, err)
		return
	}
	s.completeSignIn(client, approval)
}

// recordDeviceEvent shows the code and the page as soon as the backend issues them.
func (s *Session) recordDeviceEvent(event backend.DeviceEvent) {
	if event.Phase != backend.DevicePending {
		return
	}
	s.mu.Lock()
	s.sign = SignInState{
		Phase:           PhasePending,
		Code:            event.SignIn.Code,
		VerificationURL: event.SignIn.VerificationURL,
		ExpiresAt:       event.SignIn.ExpiresAt,
	}
	s.mu.Unlock()
	s.notifyChange()
}

// openVerificationPage is the device flow's browser step. A failure is shown in the view, never as an error.
func (s *Session) openVerificationPage(url string) {
	err := s.openPage(url)
	s.mu.Lock()
	s.sign.BrowserOpenFailed = err != nil
	s.mu.Unlock()
	s.notifyChange()
}

// openPage opens url in the browser, or reports that there is no browser to open it in.
func (s *Session) openPage(url string) error {
	if s.deps.Browser == nil {
		s.deps.Logger.Warn("auth.browser_unavailable", nil)
		return ErrNoVerificationPage
	}
	if err := s.deps.Browser(url); err != nil {
		s.deps.Logger.Warn("auth.browser_open_failed", nil)
		return err
	}
	return nil
}

// completeSignIn stores the approved token in the keychain, records the connection and, when client is still the
// active backend, shows the account as signed in. A token approved for a backend the user has since left is still
// stored for that backend.
func (s *Session) completeSignIn(client *backend.Client, approval backend.Approval) {
	origin := client.Environment().BaseURL
	if err := s.vault.SaveToken(origin, approval.Token); err != nil {
		s.deps.Logger.Warn("auth.token_save_failed", logx.F{"error": err.Error()})
		s.endFlow(client, SignInState{Phase: PhaseFailed, Message: keychainFailedMessage})
		return
	}
	connection := Connection{
		UserID:      approval.User.ID,
		Email:       approval.User.Email,
		Role:        approval.User.Role,
		ConnectedAt: s.timestamp(),
	}
	s.mu.Lock()
	s.persistLocked(withConnection(origin, connection))
	if s.client == client {
		s.connected = true
		s.connection = connection
		s.account = AccountInfo{}
		s.sign = SignInState{Phase: PhaseApproved}
		s.cancelSignIn = nil
	}
	s.mu.Unlock()
	s.deps.Logger.Info("auth.signin_approved", nil)
	s.notifyChange()
}

// endFlowFailed records a flow that ended without a token, mapping the error to the state the view shows.
func (s *Session) endFlowFailed(ctx context.Context, client *backend.Client, err error) {
	state := SignInState{Phase: PhaseFailed, Message: signInFailedMessage}
	switch {
	case errors.Is(err, backend.ErrSignInDenied):
		state = SignInState{Phase: PhaseDenied}
	case errors.Is(err, backend.ErrSignInExpired):
		state = SignInState{Phase: PhaseExpired}
	case ctx.Err() != nil || backend.IsKind(err, backend.KindCanceled):
		state = SignInState{Phase: PhaseCancelled}
	default:
		s.deps.Logger.Warn("auth.signin_failed", logx.F{"kind": errorKind(err)})
	}
	s.endFlow(client, state)
}

// endFlow sets the state of a finished flow. A flow for a backend that is no longer active leaves the view alone,
// and the flow claim is released only by the flow that holds it.
func (s *Session) endFlow(client *backend.Client, state SignInState) {
	s.mu.Lock()
	if s.client == client {
		s.sign = state
		s.cancelSignIn = nil
	}
	s.mu.Unlock()
	s.notifyChange()
}

// withConnection returns a profile change that records connection for origin.
func withConnection(origin string, connection Connection) func(*Profile) {
	return func(p *Profile) {
		if p.Connections == nil {
			p.Connections = map[string]Connection{}
		}
		p.Connections[origin] = connection
	}
}

// timestamp is the current time for stored records. It reads the injected clock, never the wall clock.
func (s *Session) timestamp() string {
	return s.deps.Clock.Now().UTC().Format(time.RFC3339)
}

// errorKind names the backend error kind for logs, or "other" for an error the backend client did not classify.
func errorKind(err error) string {
	var typed *backend.Error
	if errors.As(err, &typed) {
		return string(typed.Kind)
	}
	return "other"
}
